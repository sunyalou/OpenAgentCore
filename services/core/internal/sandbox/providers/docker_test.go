package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/sandbox"
	"github.com/google/uuid"
)

func TestDockerMountSourcesMustBeAvailable(t *testing.T) {
	root := t.TempDir()
	seccomp := filepath.Join(root, "seccomp.json")
	if err := os.WriteFile(seccomp, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	config := func(source string) Config {
		return Config{
			Generation:     1,
			Provider:       "docker",
			InstallationID: uuid.NewString(),
			Specification:  sandbox.DeploymentSpec{Resources: sandbox.Resources{CPUs: 2, MemoryMiB: 2048}},
			Docker: &Docker{
				Host: "unix:///var/run/docker.sock", Image: "test@sha256:" + strings.Repeat("a", 64),
				Network: "bridge", SeccompFile: seccomp, Mounts: []Mount{{Source: source, Target: "/opt/xre"}},
			},
		}
	}
	if closeProvider, err := buildDocker(config(filepath.Join(root, "missing")), LocalOptions{}, &Built{}); err == nil {
		closeProvider()
		t.Fatal("accepted an unavailable mount source")
	}
	if closeProvider, err := buildDocker(config(root), LocalOptions{}, &Built{}); err != nil {
		t.Fatalf("rejected an available mount source: %v", err)
	} else {
		closeProvider()
	}
}

func TestDockerUlimitsRequireBounds(t *testing.T) {
	root := t.TempDir()
	seccomp := filepath.Join(root, "seccomp.json")
	if err := os.WriteFile(seccomp, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	soft, hard := int64(-1), int64(-1)
	config := func(ulimits []Ulimit) Config {
		return Config{
			Generation:     1,
			Provider:       "docker",
			InstallationID: uuid.NewString(),
			Specification:  sandbox.DeploymentSpec{Resources: sandbox.Resources{CPUs: 2, MemoryMiB: 2048}},
			Docker: &Docker{
				Host: "unix:///var/run/docker.sock", Image: "test@sha256:" + strings.Repeat("a", 64),
				Network: "bridge", SeccompFile: seccomp, Ulimits: ulimits,
			},
		}
	}
	if closeProvider, err := buildDocker(config([]Ulimit{{Name: "memlock"}}), LocalOptions{}, &Built{}); err == nil {
		closeProvider()
		t.Fatal("accepted a ulimit without soft and hard values")
	}
	for _, partial := range []Ulimit{{Name: "memlock", Soft: &soft}, {Name: "memlock", Hard: &hard}} {
		if closeProvider, err := buildDocker(config([]Ulimit{partial}), LocalOptions{}, &Built{}); err == nil {
			closeProvider()
			t.Fatal("accepted a ulimit with only one of soft and hard")
		}
	}
	if closeProvider, err := buildDocker(config([]Ulimit{{Name: "memlock", Soft: &soft, Hard: &hard}}), LocalOptions{}, &Built{}); err != nil {
		t.Fatalf("rejected a complete ulimit: %v", err)
	} else {
		closeProvider()
	}
}
