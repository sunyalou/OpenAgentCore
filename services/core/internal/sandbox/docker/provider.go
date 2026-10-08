// Package docker is a thin SDK adapter for the dedicated, colocated Runtime.
package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/MiniMax-AI/OpenAgentCore/internal/agentnetwork"
	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/sandbox"
	"github.com/containerd/errdefs"
	"github.com/google/uuid"
	"github.com/moby/moby/client"
)

const labelPrefix = "io.oac."

// Config is trusted operator configuration, never public Session input. The
// immutable image contains the qualified native profile and all Runtime binaries.
// Seccomp is JSON content, not a path on the Docker host. Network must provide
// trusted daemon/model connectivity; it is the node's Docker network or `host`,
// which shares the host's network stack. Native tool network policy is in the image.
// Devices and Mounts are host passthroughs the operator opts into per node; they
// widen what a sandbox can reach and must only be configured on trusted hosts.
// ShmSizeMiB and PidsLimit are operator resource overrides for every container.
type Config struct {
	InstallationID, Image, Network, Seccomp string
	ExtraHosts                              []string
	NestedSandbox                           bool
	Devices                                 []string
	Mounts                                  []Mount
	Ulimits                                 []Ulimit
	ShmSizeMiB                              *int64
	PidsLimit                               *int64
	Resources                               *sandbox.Resources
}

// Mount is one read-only host path exposed inside every Runtime container.
type Mount struct{ Source, Target string }

// Ulimit is one resource limit applied to every Runtime container.
type Ulimit struct {
	Name       string
	Soft, Hard int64
}
type Provider struct {
	client *client.Client
	config Config
}

var _ sandbox.SandboxProvider = (*Provider)(nil)

func New(c *client.Client, config Config) (*Provider, error) {
	if c == nil || !validID(config.InstallationID) || (!strings.HasPrefix(config.Image, "sha256:") && !strings.Contains(config.Image, "@sha256:")) || config.Seccomp == "" || config.Network == "" || strings.HasPrefix(config.Network, "container:") || !validDevices(config.Devices) || !validMounts(config.Mounts) || !validUlimits(config.Ulimits) || !validShmSize(config.ShmSizeMiB) || !validPidsLimit(config.PidsLimit) {
		return nil, sandbox.ErrInvalid
	}
	if config.Resources != nil {
		if ValidateResources(*config.Resources) != nil {
			return nil, sandbox.ErrInvalid
		}
		resources := *config.Resources
		config.Resources = &resources
	}
	return &Provider{client: c, config: config}, nil
}
func validID(v string) bool {
	u, e := uuid.Parse(v)
	return e == nil && u != uuid.Nil && u.String() == v
}

const (
	maxDevices    = 64
	maxMounts     = 16
	maxUlimits    = 16
	maxShmSizeMiB = 1048576
	maxPidsLimit  = 1048576
)

// validDevices accepts deduplicated canonical device paths under /dev.
func validDevices(devices []string) bool {
	if len(devices) > maxDevices {
		return false
	}
	seen := map[string]bool{}
	for _, device := range devices {
		if !strings.HasPrefix(device, "/dev/") || path.Clean(device) != device || seen[device] {
			return false
		}
		seen[device] = true
	}
	return true
}

// validMounts accepts canonical absolute source and target paths whose target
// never shadows a Runtime-owned path. Host mounts are always read-only.
func validMounts(mounts []Mount) bool {
	if len(mounts) > maxMounts {
		return false
	}
	seen := map[string]bool{}
	for _, m := range mounts {
		if !validAbsolutePath(m.Source) || !validMountTarget(m.Target) || seen[m.Target] {
			return false
		}
		seen[m.Target] = true
	}
	return true
}

func validAbsolutePath(v string) bool { return path.IsAbs(v) && path.Clean(v) == v && v != "/" }

func validMountTarget(v string) bool {
	if !validAbsolutePath(v) {
		return false
	}
	for _, reserved := range []string{"/proc", "/sys", "/dev", "/home", "/environment", "/workspace", "/tmp"} {
		if v == reserved || strings.HasPrefix(v, reserved+"/") {
			return false
		}
	}
	return true
}

// validUlimits accepts uniquely named resource limits; -1 means unlimited.
func validUlimits(ulimits []Ulimit) bool {
	if len(ulimits) > maxUlimits {
		return false
	}
	seen := map[string]bool{}
	for _, u := range ulimits {
		if !validUlimitName(u.Name) || seen[u.Name] || u.Soft < -1 || u.Hard < -1 || (u.Hard != -1 && (u.Soft == -1 || u.Soft > u.Hard)) {
			return false
		}
		seen[u.Name] = true
	}
	return true
}

func validUlimitName(name string) bool {
	if name == "" || len(name) > 32 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// validShmSize accepts an optional /dev/shm size in MiB.
func validShmSize(mib *int64) bool { return mib == nil || (*mib >= 1 && *mib <= maxShmSizeMiB) }

// validPidsLimit accepts an optional task limit.
func validPidsLimit(limit *int64) bool {
	return limit == nil || (*limit >= 1 && *limit <= maxPidsLimit)
}
func validReference(r sandbox.Reference) bool {
	return validID(r.TenantID) && validID(r.EnvironmentID) && validID(r.AllocationID)
}
func (p *Provider) name(r sandbox.Reference) string {
	h := sha256.Sum256([]byte(p.config.InstallationID + ":" + r.TenantID + ":" + r.EnvironmentID + ":" + r.AllocationID))
	return "oac-runtime-" + hex.EncodeToString(h[:16])
}
func (p *Provider) labels(r sandbox.Reference) map[string]string {
	return map[string]string{
		labelPrefix + "installation": p.config.InstallationID, labelPrefix + "tenant": r.TenantID, labelPrefix + "environment": r.EnvironmentID, labelPrefix + "allocation": r.AllocationID,
	}
}
func (p *Provider) owns(labels map[string]string, r sandbox.Reference) bool {
	for k, v := range p.labels(r) {
		if labels[k] != v {
			return false
		}
	}
	return true
}
func (p *Provider) inspect(ctx context.Context, r sandbox.Reference) (client.ContainerInspectResult, error) {
	if !validReference(r) {
		return client.ContainerInspectResult{}, sandbox.ErrInvalid
	}
	v, e := p.client.ContainerInspect(ctx, p.name(r), client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		return v, sandbox.ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if v.Container.Config == nil || !p.owns(v.Container.Config.Labels, r) {
		return v, sandbox.ErrOwnership
	}
	return v, nil
}
func (p *Provider) GetInfo(ctx context.Context, r sandbox.Reference) (sandbox.Info, error) {
	info := sandbox.Info{Reference: r}
	v, e := p.inspect(ctx, r)
	if e != nil {
		return info, e
	}
	if e = p.verifyConfiguration(ctx, v); e != nil {
		return info, e
	}
	info.ProviderID = v.Container.ID
	info.State = string(v.Container.State.Status)
	info.BootstrapComplete = info.State == "running"
	return info, nil
}

// Renew observes Docker state. Docker has no lease; service-owned expiry must be
// managed durably by Core. Never synthesize an expiry or revive a stopped Runtime.
func (p *Provider) Renew(ctx context.Context, r sandbox.Reference) (sandbox.Info, error) {
	return p.GetInfo(ctx, r)
}

func (p *Provider) Create(ctx context.Context, b sandbox.Bootstrap) (sandbox.Info, error) {
	info := sandbox.Info{Reference: b.Reference}
	policy := agentnetwork.Policy{Access: b.NetworkAccess, AllowedDomains: b.AllowedDomains}
	if policy.Validate() != nil || !validReference(b.Reference) || !validID(b.SessionID) || !validID(b.DeviceID) || b.RuntimeConnection().Validate() != nil {
		return info, sandbox.ErrInvalid
	}
	if existing, e := p.GetInfo(ctx, b.Reference); e == nil {
		return existing, sandbox.ErrExists
	} else if !errors.Is(e, sandbox.ErrNotFound) {
		return info, e
	}
	domains, err := json.Marshal(policy.Hosts())
	if err != nil {
		return info, sandbox.ErrInvalid
	}
	name := p.name(b.Reference)
	// Retained volumes without a container are partial or lost state, not an
	// invitation to overwrite native history with a new bootstrap identity.
	for _, suffix := range []string{"-home", "-environment"} {
		v, err := p.client.VolumeInspect(ctx, name+suffix, client.VolumeInspectOptions{})
		if err == nil {
			if !p.owns(v.Volume.Labels, b.Reference) {
				return info, sandbox.ErrOwnership
			}
			return info, sandbox.ErrExists
		}
		if !errdefs.IsNotFound(err) {
			return info, err
		}
	}
	for _, suffix := range []string{"-home", "-environment"} {
		v, e := p.client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name + suffix, Labels: p.labels(b.Reference)})
		if e != nil {
			return info, e
		}
		if !p.owns(v.Volume.Labels, b.Reference) {
			return info, sandbox.ErrOwnership
		}
	}
	options := runtimeContainerOptions(p.config, name, p.labels(b.Reference), []string{"OAC_RUNTIME_ENVIRONMENT_ID=" + b.EnvironmentID, "OAC_RUNTIME_SESSION_ID=" + b.SessionID, "OAC_RUNTIME_NETWORK_ACCESS=" + policy.Access, "OAC_RUNTIME_ALLOWED_DOMAINS=" + string(domains)})
	options.Config.Cmd = []string{"connect", "--profile", "default", "--bootstrap-file", "/home/runtime/runtime-bootstrap.json"}
	v, e := p.client.ContainerCreate(ctx, options)
	if errdefs.IsConflict(e) {
		return info, sandbox.ErrExists
	}
	if e != nil {
		return info, e
	}
	info.ProviderID = v.ID
	if p.config.Resources != nil {
		actual, err := p.inspect(ctx, b.Reference)
		if err != nil {
			info.CreateSettled = true
			return info, err
		}
		if err = p.verifyConfiguration(ctx, actual); err != nil {
			// Container creation completed and bootstrap has not started. No
			// outstanding mutation can recreate resources after owned cleanup.
			info.CreateSettled = true
			return info, err
		}
	}
	// Any failure returns the retained allocation reference. The caller must Kill
	// it, including on a lost acknowledgement. Never erase uncertain owner state.
	if e = p.bootstrap(ctx, v.ID, b); e != nil {
		return info, fmt.Errorf("runtime bootstrap: %w", e)
	}
	if _, e = p.client.ContainerStart(ctx, v.ID, client.ContainerStartOptions{}); e != nil {
		return info, e
	}
	return p.GetInfo(ctx, b.Reference)
}

// Kill is idempotent only for absence, not for errors or foreign ownership. It
// checks all resources before removing any and confirms removal of named volumes.
func (p *Provider) Kill(ctx context.Context, r sandbox.Reference) error {
	if !validReference(r) {
		return sandbox.ErrInvalid
	}
	c, e := p.inspect(ctx, r)
	if e != nil && !errors.Is(e, sandbox.ErrNotFound) {
		return e
	}
	present := e == nil
	name := p.name(r)
	volumes := []string{}
	for _, suffix := range []string{"-home", "-environment"} {
		v, e := p.client.VolumeInspect(ctx, name+suffix, client.VolumeInspectOptions{})
		if errdefs.IsNotFound(e) {
			continue
		}
		if e != nil {
			return e
		}
		if !p.owns(v.Volume.Labels, r) {
			return sandbox.ErrOwnership
		}
		volumes = append(volumes, name+suffix)
	}
	if present {
		if _, e = p.client.ContainerRemove(ctx, c.Container.ID, client.ContainerRemoveOptions{Force: true}); e != nil && !errdefs.IsNotFound(e) {
			return e
		}
	}
	for _, v := range volumes {
		if _, e = p.client.VolumeRemove(ctx, v, client.VolumeRemoveOptions{}); e != nil && !errdefs.IsNotFound(e) {
			return e
		}
	}
	if _, e = p.inspect(ctx, r); !errors.Is(e, sandbox.ErrNotFound) {
		if e == nil {
			return errors.New("container removal unconfirmed")
		}
		return e
	}
	for _, v := range volumes {
		if _, e = p.client.VolumeInspect(ctx, v, client.VolumeInspectOptions{}); !errdefs.IsNotFound(e) {
			if e == nil {
				return errors.New("volume removal unconfirmed")
			}
			return e
		}
	}
	return nil
}
