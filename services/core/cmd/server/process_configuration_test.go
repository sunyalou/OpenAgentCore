package main

import (
	"os"
	"strings"
	"testing"

	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/processconfig"
)

func TestExecutionConcurrencyConfiguration(t *testing.T) {
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "unused")
	if err := os.Unsetenv("OAC_EXECUTION_CONCURRENCY"); err != nil {
		t.Fatal(err)
	}
	if got, err := processconfig.ExecutionConcurrency(); err != nil || got != 4 {
		t.Fatal(got, err)
	}
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "")
	if got, err := processconfig.ExecutionConcurrency(); err != nil || got != 4 {
		t.Fatal("empty concurrency did not keep the default", got, err)
	}
	for _, value := range []string{"1", "7", "1024"} {
		t.Setenv("OAC_EXECUTION_CONCURRENCY", value)
		if got, err := processconfig.ExecutionConcurrency(); err != nil || got < 1 {
			t.Fatal(value, got, err)
		}
	}
	for _, value := range []string{"0", "-1", "1025", "1.5", "secret-value"} {
		t.Setenv("OAC_EXECUTION_CONCURRENCY", value)
		if _, err := processconfig.ExecutionConcurrency(); err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatal("invalid concurrency accepted or echoed", err)
		}
	}
}

func TestPublicURLMustBeACanonicalOrigin(t *testing.T) {
	for _, value := range []string{"https://core.example", "https://core.example:8443", "http://127.0.0.1:8091"} {
		t.Setenv("OAC_PUBLIC_URL", value)
		if got, err := processconfig.PublicURL(); err != nil || got != value {
			t.Fatal(value, got, err)
		}
	}
	for _, value := range []string{"https://core.example/", "https://Core.example", "http://core.example", "wss://core.example", "https://core.example/v1"} {
		t.Setenv("OAC_PUBLIC_URL", value)
		if _, err := processconfig.PublicURL(); err == nil {
			t.Fatal("accepted", value)
		}
	}
}

func TestPublicURLDrivesInsecureWebSocketURL(t *testing.T) {
	const insecure = "http://10.0.0.5:8080"
	t.Setenv("OAC_PUBLIC_URL", insecure)
	if _, err := processconfig.PublicURL(); err == nil {
		t.Fatal("accepted a non-loopback HTTP origin with OAC_ALLOW_INSECURE_ORIGIN unset")
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	public, err := processconfig.PublicURL()
	if err != nil || public != insecure {
		t.Fatalf("PublicURL() = %q, %v", public, err)
	}
	if ws, err := runtimeWebSocketURL(public); err != nil || ws != "ws://10.0.0.5:8080/api/v1/agent-daemon/ws" {
		t.Fatalf("runtimeWebSocketURL(%q) = %q, %v", public, ws, err)
	}
}

// installationFacts backs GET /core/v1/installation; the switch must reach the
// settings snapshot the console reads.
func TestInstallationFactsReportAllowInsecureOrigin(t *testing.T) {
	t.Setenv("OAC_PUBLIC_URL", "http://10.0.0.5:8080")
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	facts, err := installationFacts("http://10.0.0.5:8080")
	if err != nil {
		t.Fatal(err)
	}
	if facts.Configuration == nil {
		t.Fatal("installation facts omitted the settings snapshot")
	}
	found := false
	for _, setting := range facts.Configuration.Settings {
		if setting.Key == "allow_insecure_origin" {
			found = true
			if setting.Value != true || setting.Default != false || setting.Sensitive {
				t.Fatalf("allow_insecure_origin = %+v", setting)
			}
		}
	}
	if !found {
		t.Fatal("installation facts omitted allow_insecure_origin")
	}
}
