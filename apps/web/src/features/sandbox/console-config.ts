/** A provider whose nodes install from files this console serves. */
export type NodeArtifactProvider = "docker" | "microsandbox";

export interface SandboxConsoleConfig {
  sandbox_admin: boolean;
  node_installer: boolean;
  node_installer_sha256: string;
  /**
   * The installation's `allow_insecure_origin` switch, as the console derives it
   * from its environment. Absent when the console does not report it (an older
   * console), which reads as off.
   */
  allow_insecure_origin?: boolean;
  /**
   * The providers whose node files this console serves. Absent when the console
   * does not report them (an older console), which blocks nothing; a reported
   * null or malformed value reads as none.
   */
  node_artifacts?: NodeArtifactProvider[];
}

const SHA256 = /^[a-f0-9]{64}$/;

/**
 * The console's capability flags. Signing in with the Core key grants
 * administration, so Core reports only its node installer and digest and the providers it has node files for;
 * sandbox administration is available
 * unless the console says `sandbox_admin: false`. An installer is offered only
 * with a well-formed SHA-256 digest.
 * An absent endpoint (404, an older console) means no sandbox administration;
 * any other failure is thrown so callers report a failed read instead of
 * "not configured".
 */
export async function sandboxConsoleConfig(signal: AbortSignal): Promise<SandboxConsoleConfig | null> {
  const response = await fetch("/console/config", { credentials: "include", signal });
  if (response.status === 404) return null;
  if (!response.ok) throw new Error(`The console configuration could not be read (HTTP ${response.status}).`);
  const config = await response.json() as Partial<Omit<SandboxConsoleConfig, "node_artifacts">> & { node_artifacts?: unknown };
  return {
    sandbox_admin: config.sandbox_admin !== false,
    node_installer: config.node_installer === true && SHA256.test(config.node_installer_sha256 ?? ""),
    node_installer_sha256: config.node_installer_sha256 ?? "",
    allow_insecure_origin: config.allow_insecure_origin === true,
    ...(config.node_artifacts === undefined ? {} : { node_artifacts: nodeArtifacts(config.node_artifacts) }),
  };
}

/** A reported list keeps the providers it names; null or any other value means none. */
function nodeArtifacts(value: unknown): NodeArtifactProvider[] {
  return Array.isArray(value) ? value.filter((entry): entry is NodeArtifactProvider => entry === "docker" || entry === "microsandbox") : [];
}

/** Whether a node of this provider can install from the console's files; true when the console doesn't report them. */
export function nodeFilesAvailable(config: SandboxConsoleConfig, provider: string): boolean {
  return config.node_artifacts === undefined || config.node_artifacts.some((entry) => entry === provider);
}
