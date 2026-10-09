import type { CoreInstallation } from "@oac/agents-client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { installationQuery } from "../../lib/installation";
import { NodeCleanupDialog, type NodeCleanup } from "./NodeCleanupDialog";

const cleanup: NodeCleanup = {
  name: "edge-01", installationId: "7f3c2a90-5b1e-4c2d-9e3f-0a1b2c3d4e5f",
  scriptDigest: "a".repeat(64), provider: "docker", oldAddress: null,
};

/** The installation Add node used: `allowInsecure` undefined means the snapshot predates the setting. */
function installation(allowInsecure: boolean | undefined, publicUrl: string | null): CoreInstallation {
  return {
    object: "core.installation", installation_id: "7f3c2a90-5b1e-4c2d-9e3f-0a1b2c3d4e5f",
    public_url: publicUrl, api_base_url: publicUrl === null ? null : `${publicUrl}/v1`, local_only: false,
    source_commit: "c".repeat(40),
    configuration: {
      path: "/opt/oac/config.json", apply_command: "sudo oac apply", applied_at: "2026-01-01T00:00:00Z",
      settings: allowInsecure === undefined ? [] : [{ key: "allow_insecure_origin", value: allowInsecure, default: false, changeable: true, sensitive: false, restarts: ["core"] }],
    },
    address_bindings: { nodes: 1, nodes_on_other_address: 0, hosted_sandboxes: 0, self_hosted_executors: 0 },
  };
}

function render(data: CoreInstallation): string {
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  cache.setQueryData(installationQuery.queryKey, data);
  const html = renderToStaticMarkup(<QueryClientProvider client={cache}><NodeCleanupDialog cleanup={cleanup} open onClose={() => {}} /></QueryClientProvider>);
  cache.clear();
  return html;
}

describe("the host uninstall command under allow_insecure_origin", () => {
  it("generates the command for a non-loopback plain-HTTP public URL when the switch is on", () => {
    const html = render(installation(true, "http://10.0.0.5:8080"));
    expect(html).toContain("http://10.0.0.5:8080/node-install/node-install.pyz");
    expect(html).toContain("--uninstall --installation-id");
    // The operator's internal CA reaches curl as --cacert when the shell sets OAC_CORE_CA; the installer reads it from the retained identity.
    expect(html).toContain("${OAC_CORE_CA:+ --cacert");
  });
  it("keeps the HTTPS requirement and issues no command when the switch is off", () => {
    const html = render(installation(false, "http://10.0.0.5:8080"));
    expect(html).not.toContain("node-install.pyz");
    expect(html).toContain("An uninstall command needs an HTTPS public URL that other machines can reach, and this installation has none.");
  });
  it("keeps the HTTPS requirement when the snapshot predates the setting", () => {
    const html = render(installation(undefined, "http://10.0.0.5:8080"));
    expect(html).not.toContain("node-install.pyz");
    expect(html).toContain("An uninstall command needs an HTTPS public URL that other machines can reach, and this installation has none.");
  });
  it("drops only the HTTPS wording when the switch is on but no public URL is usable", () => {
    const html = render(installation(true, null));
    expect(html).toContain("An uninstall command needs a public URL that other machines can reach, and this installation has none.");
    expect(html).not.toContain("An uninstall command needs an HTTPS public URL");
  });
});
