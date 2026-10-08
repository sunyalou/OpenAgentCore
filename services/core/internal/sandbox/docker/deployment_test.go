package docker

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/sandbox"
	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func TestManagedDriftRejectsBeforeBootstrapAndRetainsCleanup(t *testing.T) {
	for _, drift := range []string{"cpu", "memory", "image"} {
		t.Run(drift, func(t *testing.T) {
			imageID := "sha256:" + strings.Repeat("a", 64)
			present := false
			volumes := map[string]map[string]any{}
			var created struct {
				container.Config
				HostConfig container.HostConfig
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/v1.52")
				w.Header().Set("Content-Type", "application/json")
				switch {
				case path == "/containers/create":
					_ = json.NewDecoder(r.Body).Decode(&created)
					if created.HostConfig.NanoCPUs != 3e9 || created.HostConfig.Memory != 4096*1024*1024 {
						t.Error("create did not apply managed limits")
					}
					present = true
					w.WriteHeader(201)
					_, _ = io.WriteString(w, `{"Id":"runtime-id"}`)
				case strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json") && present:
					host := created.HostConfig
					actualImage := imageID
					switch drift {
					case "cpu":
						host.NanoCPUs = 1e9
					case "memory":
						host.Memory = 1024 * 1024 * 1024
					case "image":
						actualImage = "sha256:" + strings.Repeat("b", 64)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"Id": "runtime-id", "Image": actualImage, "Config": created.Config, "HostConfig": host, "State": map[string]any{"Status": "created"}})
				case strings.HasPrefix(path, "/images/"):
					_ = json.NewEncoder(w).Encode(map[string]string{"Id": imageID})
				case path == "/volumes/create":
					var value map[string]any
					_ = json.NewDecoder(r.Body).Decode(&value)
					volumes[value["Name"].(string)] = value
					_ = json.NewEncoder(w).Encode(value)
				case strings.HasPrefix(path, "/volumes/") && volumes[strings.TrimPrefix(path, "/volumes/")] != nil:
					name := strings.TrimPrefix(path, "/volumes/")
					if r.Method == http.MethodDelete {
						delete(volumes, name)
						w.WriteHeader(204)
					} else {
						_ = json.NewEncoder(w).Encode(volumes[name])
					}
				case r.Method == http.MethodDelete && strings.HasPrefix(path, "/containers/"):
					present = false
					w.WriteHeader(204)
				case r.Method == http.MethodGet:
					w.WriteHeader(404)
					_, _ = io.WriteString(w, `{"message":"not found"}`)
				default:
					t.Errorf("unexpected bootstrap or mutation: %s %s", r.Method, path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			c, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.52"))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			resources := sandbox.Resources{CPUs: 3, MemoryMiB: 4096}
			p, err := New(c, Config{InstallationID: uuid.NewString(), Image: imageID, Network: "bridge", Seccomp: `{}`, Resources: &resources})
			if err != nil {
				t.Fatal(err)
			}
			resources.CPUs = 9
			b := sandbox.Bootstrap{Reference: sandbox.Reference{TenantID: uuid.NewString(), EnvironmentID: uuid.NewString(), AllocationID: uuid.NewString()}, SessionID: uuid.NewString(), DeviceID: uuid.NewString(), CoreURL: "https://core.example/api/v1", Credential: "secret", NetworkAccess: "enabled"}
			info, err := p.Create(t.Context(), b)
			if !errors.Is(err, sandbox.ErrInvalid) {
				t.Fatal("drift accepted", err)
			}
			if info.Reference != b.Reference || !info.CreateSettled || info.BootstrapComplete || info.ProviderID == "" || info.State == "absent" {
				t.Fatal("pre-bootstrap rejection lost its creation settlement", info)
			}
			if _, err = p.GetInfo(t.Context(), b.Reference); !errors.Is(err, sandbox.ErrInvalid) {
				t.Fatal("drift readiness accepted", err)
			}
			if err = p.Kill(t.Context(), b.Reference); err != nil {
				t.Fatal("drift prevented cleanup", err)
			}
			if present || len(volumes) != 0 {
				t.Fatal("cleanup retained resources")
			}
		})
	}
}

func TestNilManagedLimitsKeepCallerManagedDefaults(t *testing.T) {
	options := runtimeContainerOptions(Config{}, "fixture", nil, nil)
	if options.HostConfig.NanoCPUs != 2e9 || options.HostConfig.Memory != 2*1024*1024*1024 {
		t.Fatal("caller-managed defaults changed")
	}
}

func TestRuntimeContainerOptionsCarryShmSizeAndPidsLimit(t *testing.T) {
	shm, pids := int64(131072), int64(4096)
	options := runtimeContainerOptions(Config{ShmSizeMiB: &shm, PidsLimit: &pids}, "fixture", nil, nil)
	if options.HostConfig.ShmSize != 131072*1024*1024 || options.HostConfig.Resources.PidsLimit == nil || *options.HostConfig.Resources.PidsLimit != 4096 {
		t.Fatal("shm size or pids limit is not applied")
	}
	defaults := runtimeContainerOptions(Config{}, "fixture", nil, nil)
	if defaults.HostConfig.ShmSize != 0 || defaults.HostConfig.Resources.PidsLimit == nil || *defaults.HostConfig.Resources.PidsLimit != 128 {
		t.Fatal("container resource defaults changed")
	}
}

func TestRuntimeContainerOptionsCarryUlimits(t *testing.T) {
	options := runtimeContainerOptions(Config{Ulimits: []Ulimit{{Name: "memlock", Soft: -1, Hard: -1}}}, "fixture", nil, nil)
	ulimits := options.HostConfig.Resources.Ulimits
	if len(ulimits) != 1 || ulimits[0] == nil || ulimits[0].Name != "memlock" || ulimits[0].Soft != -1 || ulimits[0].Hard != -1 {
		t.Fatal("ulimits are not applied")
	}
}

func TestRuntimeContainerOptionsHostNetwork(t *testing.T) {
	options := runtimeContainerOptions(Config{Network: "host"}, "fixture", nil, nil)
	if options.HostConfig.NetworkMode != "host" {
		t.Fatal("network mode is not passed through")
	}
}

func TestRuntimeContainerOptionsCarryDevicesAndMounts(t *testing.T) {
	options := runtimeContainerOptions(Config{Devices: []string{"/dev/xpu0", "/dev/xpuctrl"}, Mounts: []Mount{{Source: "/opt/xre", Target: "/opt/xre"}}}, "fixture", nil, nil)
	if len(options.HostConfig.Devices) != 2 || options.HostConfig.Devices[0].PathOnHost != "/dev/xpu0" || options.HostConfig.Devices[0].PathInContainer != "/dev/xpu0" || options.HostConfig.Devices[0].CgroupPermissions != "rwm" || options.HostConfig.Devices[1].PathOnHost != "/dev/xpuctrl" {
		t.Fatal("devices are not passed through")
	}
	found := false
	volumes := 0
	for _, m := range options.HostConfig.Mounts {
		if m.Type == mount.TypeVolume {
			volumes++
		}
		if m.Type == mount.TypeBind && m.Source == "/opt/xre" && m.Target == "/opt/xre" && m.ReadOnly {
			found = true
		}
	}
	if volumes != 3 {
		t.Fatal("volume layout changed")
	}
	if !found {
		t.Fatal("host mount is not exposed read-only")
	}
}
