package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRejectsUnsafeURLsAndSecretFiles(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "core.key")
	valid := strings.Repeat("k", 32)
	if err := os.WriteFile(key, []byte(valid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_WEB_CORE_KEY_FILE", key)
	t.Setenv("OAC_WEB_ORIGIN", testOrigin)
	t.Setenv("OAC_WEB_UPSTREAM", "http://core:8091")
	t.Setenv("OAC_WEB_DIST", directory)
	c, err := loadConfig()
	if err != nil || c.coreKey != valid {
		t.Fatalf("valid configuration failed: %v", err)
	}
	for _, value := range []string{"http://user:secret@core:8091", "http://core:8091/v1", "http://core:8091?token=secret", "http://core:8091#", "file:///config/caller.key", ""} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OAC_WEB_UPSTREAM", value)
			_, err := loadConfig()
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatal("unsafe URL accepted or exposed")
			}
		})
	}
	for _, value := range []string{"", "token with spaces", strings.Repeat("x", 4097), "token\x00", strings.Repeat("s", 31)} {
		if err := os.WriteFile(key, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfig(); err == nil || strings.Contains(err.Error(), "sss") {
			t.Fatal("invalid Core key accepted or echoed")
		}
	}
	if err := os.WriteFile(key, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(); err == nil {
		t.Fatal("publicly readable secret accepted")
	}
}

// OAC_ALLOW_INSECURE_ORIGIN is the installer's projection of config.json's
// allow_insecure_origin; the console only reads it, and only 0 or 1 are valid.
func TestConfigReadsAllowInsecureOrigin(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "core.key")
	if err := os.WriteFile(key, []byte(strings.Repeat("k", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_WEB_CORE_KEY_FILE", key)
	t.Setenv("OAC_WEB_ORIGIN", testOrigin)
	t.Setenv("OAC_WEB_UPSTREAM", "http://core:8091")
	t.Setenv("OAC_WEB_DIST", directory)
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	if c, err := loadConfig(); err != nil || !c.allowInsecureOrigin {
		t.Fatalf("allow_insecure_origin=1: %v, %v", c.allowInsecureOrigin, err)
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "0")
	if c, err := loadConfig(); err != nil || c.allowInsecureOrigin {
		t.Fatalf("allow_insecure_origin=0: %v, %v", c.allowInsecureOrigin, err)
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "yes")
	if _, err := loadConfig(); err == nil {
		t.Fatal("invalid OAC_ALLOW_INSECURE_ORIGIN accepted")
	}
}
