package processconfig

import (
	"os"
	"strings"
	"testing"
)

func TestCheckRejectsInvalidValuesWithoutEchoingThem(t *testing.T) {
	secret := "https://user:synthetic-secret@core.example"
	t.Setenv("OAC_PUBLIC_URL", secret)
	err := Check()
	if err == nil || strings.Contains(err.Error(), "synthetic-secret") || !strings.Contains(err.Error(), "OAC_PUBLIC_URL") {
		t.Fatal(err)
	}
	t.Setenv("OAC_PUBLIC_URL", "")
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "0")
	if err := Check(); err == nil || !strings.Contains(err.Error(), "OAC_EXECUTION_CONCURRENCY") || strings.Contains(err.Error(), "synthetic") {
		t.Fatal(err)
	}
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "4")
	t.Setenv("OAC_LOG_LEVEL", "verbose")
	if err := Check(); err == nil || !strings.Contains(err.Error(), "OAC_LOG_LEVEL") {
		t.Fatal(err)
	}
}

func TestSettingsReportEffectiveValuesAndHideHistory(t *testing.T) {
	t.Setenv("OAC_PUBLIC_URL", "https://core.example")
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "8")
	t.Setenv("OAC_HISTORY_SETTINGS_FILE", "/tmp/history.json")
	settings, err := Settings()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]any{}
	for _, setting := range settings {
		if setting.Sensitive && (setting.Value != nil || setting.Configured == nil) {
			t.Fatalf("sensitive setting %s leaked a value", setting.Key)
		}
		found[setting.Key] = setting.Value
		if setting.Key == "core.runtime_history" && (setting.Configured == nil || !*setting.Configured) {
			t.Fatal("history file was not reported as configured")
		}
	}
	if found["public_url"] != "https://core.example" || found["core.execution_concurrency"] != 8 {
		t.Fatal(found)
	}
}

func TestPublicURLAcceptsInsecureOriginOnlyWhenEnabled(t *testing.T) {
	const insecure = "http://10.0.0.5:8080"
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	if err := os.Unsetenv("OAC_ALLOW_INSECURE_ORIGIN"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_PUBLIC_URL", insecure)
	if _, err := PublicURL(); err == nil {
		t.Fatal("accepted a non-loopback HTTP origin with OAC_ALLOW_INSECURE_ORIGIN unset")
	} else if err.Error() != "OAC_PUBLIC_URL must be a canonical HTTPS origin without path, credentials, query or fragment, such as https://core.example; plain HTTP is accepted only for a loopback host" {
		t.Fatalf("switch-off error text changed: %v", err)
	}
	// Only the deployment side's derived value "1" enables the relaxed check.
	for _, value := range []string{"0", "true", "yes", "TRUE", "2"} {
		t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", value)
		if _, err := PublicURL(); err == nil {
			t.Fatalf("accepted a non-loopback HTTP origin with OAC_ALLOW_INSECURE_ORIGIN=%q", value)
		}
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	got, err := PublicURL()
	if err != nil || got != insecure {
		t.Fatalf("PublicURL() = %q, %v", got, err)
	}
	// A malformed origin is still rejected while the switch is on.
	t.Setenv("OAC_PUBLIC_URL", "http://Core.example")
	if _, err := PublicURL(); err == nil {
		t.Fatal("accepted a non-canonical origin with OAC_ALLOW_INSECURE_ORIGIN set")
	}
}

func TestSettingsReportAllowInsecureOrigin(t *testing.T) {
	t.Setenv("OAC_PUBLIC_URL", "https://core.example")
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	if err := os.Unsetenv("OAC_ALLOW_INSECURE_ORIGIN"); err != nil {
		t.Fatal(err)
	}
	if value := settingValue(t, "allow_insecure_origin"); value != false {
		t.Fatalf("allow_insecure_origin = %v; want false", value)
	}
	t.Setenv("OAC_ALLOW_INSECURE_ORIGIN", "1")
	if value := settingValue(t, "allow_insecure_origin"); value != true {
		t.Fatalf("allow_insecure_origin = %v; want true", value)
	}
}

func settingValue(t *testing.T, key string) any {
	t.Helper()
	settings, err := Settings()
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	t.Fatalf("setting %s is missing", key)
	return nil
}

func TestHarnessesDefaultToEveryQualifiedHarness(t *testing.T) {
	t.Setenv("OAC_DEFAULT_HARNESS", "")
	t.Setenv("OAC_HARNESSES", "")
	engineName, err := DefaultHarness()
	if err != nil || engineName != "codex" {
		t.Fatal(engineName, err)
	}
	kinds, err := Harnesses(engineName)
	if err != nil || strings.Join(kinds, ",") != "claude_sdk,codex,mcode" {
		t.Fatal(kinds, err)
	}
	t.Setenv("OAC_HARNESSES", "mcode")
	if kinds, err = Harnesses("codex"); err != nil || strings.Join(kinds, ",") != "codex,mcode" {
		t.Fatal(kinds, err)
	}
	t.Setenv("OAC_HARNESSES", "unqualified")
	if _, err = Harnesses("codex"); err == nil {
		t.Fatal("unqualified harness enabled")
	}
}
