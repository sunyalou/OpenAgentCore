package main

import (
	"os"
	"strings"
	"testing"
)

func TestExecutionConcurrencyConfiguration(t *testing.T) {
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "unused")
	if err := os.Unsetenv("OAC_EXECUTION_CONCURRENCY"); err != nil {
		t.Fatal(err)
	}
	if got, err := executionConcurrency(); err != nil || got != 4 {
		t.Fatal(got, err)
	}
	for _, value := range []string{"1", "7", "1024"} {
		t.Setenv("OAC_EXECUTION_CONCURRENCY", value)
		if got, err := executionConcurrency(); err != nil || got < 1 {
			t.Fatal(value, got, err)
		}
	}
	for _, value := range []string{"", "0", "-1", "1025", "1.5", "secret-value"} {
		t.Setenv("OAC_EXECUTION_CONCURRENCY", value)
		if _, err := executionConcurrency(); err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatal("invalid concurrency accepted or echoed", err)
		}
	}
}

func TestPublicURLMustBeACanonicalOrigin(t *testing.T) {
	for _, value := range []string{"https://core.example", "https://core.example:8443", "http://127.0.0.1:8091"} {
		t.Setenv("OAC_PUBLIC_URL", value)
		if got, err := publicURL(); err != nil || got != value {
			t.Fatal(value, got, err)
		}
	}
	for _, value := range []string{"https://core.example/", "https://Core.example", "http://core.example", "wss://core.example", "https://core.example/v1"} {
		t.Setenv("OAC_PUBLIC_URL", value)
		if _, err := publicURL(); err == nil {
			t.Fatal("accepted", value)
		}
	}
}

func TestPublicURLAcceptsInsecureOriginOnlyWhenEnabled(t *testing.T) {
	const insecure = "http://10.0.0.5:8080"
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	if err := os.Unsetenv("OAC_ALLOW_INSECURE_ORIGIN"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_PUBLIC_URL", insecure)
	if _, err := publicURL(); err == nil {
		t.Fatal("accepted a non-loopback HTTP origin with OAC_ALLOW_INSECURE_ORIGIN unset")
	}
	// Only the installer's derived value "1" enables the relaxed check.
	for _, value := range []string{"0", "true", "yes", "TRUE", "2"} {
		t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", value)
		if _, err := publicURL(); err == nil {
			t.Fatalf("accepted a non-loopback HTTP origin with OAC_ALLOW_INSECURE_ORIGIN=%q", value)
		}
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	got, err := publicURL()
	if err != nil || got != insecure {
		t.Fatalf("publicURL() = %q, %v", got, err)
	}
	if ws, err := runtimeWebSocketURL(got); err != nil || ws != "ws://10.0.0.5:8080/api/v1/agent-daemon/ws" {
		t.Fatalf("runtimeWebSocketURL(%q) = %q, %v", got, ws, err)
	}
	// A malformed origin is still rejected while the switch is on.
	t.Setenv("OAC_PUBLIC_URL", "http://Core.example")
	if _, err := publicURL(); err == nil {
		t.Fatal("accepted a non-canonical origin with OAC_ALLOW_INSECURE_ORIGIN set")
	}
}
