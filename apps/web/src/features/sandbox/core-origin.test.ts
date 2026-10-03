import type { CoreInstallationSetting } from "@oac/agents-client";
import { describe, expect, it } from "vitest";
import { allowsInsecureOrigin, httpsOrigin, nodeSourceUrl } from "./core-origin";

describe("HTTPS origin", () => {
  it.each([
    ["https://core.example.com", "https://core.example.com"],
    [" https://core.example.com:443/ ", "https://core.example.com:443"],
    ["https://10.74.84.167:18443", "https://10.74.84.167:18443"],
  ])("keeps %s as written", (input, expected) => expect(httpsOrigin(input)).toBe(expected));
  it.each(["", "/v1", "http://core.example", "http://localhost:8080", "https://core.example/v1", "https://user:secret@core.example", "https://@core.example", "https://core.example?", "https://core.example#", "https://core.example//", "https://core.example\\path", "https://co\nre.example", "file://core.example"])("rejects %s", (input) => expect(httpsOrigin(input)).toBeNull());
});

describe("node command source", () => {
  it("is the installation's public URL, never a loopback, missing or plain HTTP one", () => {
    expect(nodeSourceUrl({ public_url: "https://core.example.com:8443", local_only: false })).toBe("https://core.example.com:8443");
    expect(nodeSourceUrl({ public_url: "https://127.0.0.1:8091", local_only: true })).toBeNull();
    expect(nodeSourceUrl({ public_url: null, local_only: false })).toBeNull();
    expect(nodeSourceUrl({ public_url: "http://core.example.com", local_only: false })).toBeNull();
  });
  it("takes a non-loopback HTTP public URL only when allow_insecure_origin is on", () => {
    expect(nodeSourceUrl({ public_url: "http://10.0.0.5:8080", local_only: false }, true)).toBe("http://10.0.0.5:8080");
    // The switch never makes a loopback address reachable from another machine.
    expect(nodeSourceUrl({ public_url: "http://127.0.0.1:8091", local_only: true }, true)).toBeNull();
    // A path, credentials or a query are not an origin, switch or not.
    expect(nodeSourceUrl({ public_url: "http://10.0.0.5:8080/v1", local_only: false }, true)).toBeNull();
    expect(nodeSourceUrl({ public_url: "http://user:secret@10.0.0.5:8080", local_only: false }, true)).toBeNull();
  });
});

describe("the allow_insecure_origin switch", () => {
  const setting = (key: string, value: unknown): CoreInstallationSetting => ({ key, value, default: false, changeable: true, sensitive: false, restarts: ["core", "web"] });
  const installation = (settings: CoreInstallationSetting[] | null) => ({
    configuration: settings === null ? null : { path: "/opt/oac/config.json", apply_command: "sudo oac apply", applied_at: "2026-01-01T00:00:00Z", settings },
  });
  it("reads the installer's settings snapshot first", () => {
    expect(allowsInsecureOrigin(installation([setting("allow_insecure_origin", true)]), {})).toBe(true);
    // An explicit off in the snapshot is authoritative over a console that reports on.
    expect(allowsInsecureOrigin(installation([setting("allow_insecure_origin", false)]), { allow_insecure_origin: true })).toBe(false);
  });
  it("falls back to the console's derived switch, and is off when neither reports it", () => {
    expect(allowsInsecureOrigin(installation([setting("public_url", "https://core.example")]), { allow_insecure_origin: true })).toBe(true);
    expect(allowsInsecureOrigin(installation(null), { allow_insecure_origin: true })).toBe(true);
    expect(allowsInsecureOrigin(installation(null), {})).toBe(false);
    expect(allowsInsecureOrigin(undefined, {})).toBe(false);
  });
});
