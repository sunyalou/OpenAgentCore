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
