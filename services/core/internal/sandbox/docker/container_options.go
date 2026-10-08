package docker

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

// runtimeContainerOptions is shared by managed and user-owned V1 Runtime launch.
// Keep isolation and volume layout identical; only bootstrap authority differs.
func runtimeContainerOptions(config Config, name string, labels map[string]string, environment []string) client.ContainerCreateOptions {
	limit := int64(128)
	if config.PidsLimit != nil {
		limit = *config.PidsLimit
	}
	shmSize := int64(0)
	if config.ShmSizeMiB != nil {
		shmSize = *config.ShmSizeMiB * 1024 * 1024
	}
	memory, cpus := int64(2*1024*1024*1024), int64(2*1000000000)
	if config.Resources != nil {
		memory = int64(config.Resources.MemoryMiB) * 1024 * 1024
		cpus = int64(config.Resources.CPUs) * 1000000000
	}
	// A nested native sandbox must mount its own procfs. Keep sysfs secrets
	// masked; only this qualified image profile opts out of Docker's proc masks.
	var masked, readonly []string
	var init *bool
	if config.NestedSandbox {
		masked = []string{"/sys/firmware", "/sys/devices/virtual/powercap"}
		readonly = []string{}
		enabled := true
		init = &enabled
	}
	devices := make([]container.DeviceMapping, 0, len(config.Devices))
	for _, device := range config.Devices {
		devices = append(devices, container.DeviceMapping{PathOnHost: device, PathInContainer: device, CgroupPermissions: "rwm"})
	}
	ulimits := make([]*container.Ulimit, 0, len(config.Ulimits))
	for _, u := range config.Ulimits {
		ulimits = append(ulimits, &container.Ulimit{Name: u.Name, Soft: u.Soft, Hard: u.Hard})
	}
	mounts := []mount.Mount{
		{Type: mount.TypeVolume, Source: name + "-home", Target: "/home"},
		{Type: mount.TypeVolume, Source: name + "-environment", Target: "/environment"},
		// The native sandbox mounts canonical roots, omitting symlink aliases.
		// Expose the same workspace at its public path; trusted atomic staging
		// remains entirely on the original /environment mount.
		{Type: mount.TypeVolume, Source: name + "-environment", Target: "/workspace", VolumeOptions: &mount.VolumeOptions{Subpath: "workspace", NoCopy: true}},
	}
	for _, m := range config.Mounts {
		mounts = append(mounts, mount.Mount{Type: mount.TypeBind, Source: m.Source, Target: m.Target, ReadOnly: true})
	}
	return client.ContainerCreateOptions{Name: name, Image: config.Image,
		Config: &container.Config{User: "1000:1000", WorkingDir: "/environment/workspace", Labels: labels, Env: environment},
		HostConfig: &container.HostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges", "seccomp=" + config.Seccomp, "apparmor=unconfined"}, NetworkMode: container.NetworkMode(config.Network), ExtraHosts: config.ExtraHosts,
			MaskedPaths: masked, ReadonlyPaths: readonly, Init: init, ShmSize: shmSize,
			Resources: container.Resources{PidsLimit: &limit, Memory: memory, NanoCPUs: cpus, Devices: devices, Ulimits: ulimits}, Tmpfs: map[string]string{"/tmp": "rw,nosuid,nodev,size=128m"},
			Mounts: mounts,
		},
	}
}
