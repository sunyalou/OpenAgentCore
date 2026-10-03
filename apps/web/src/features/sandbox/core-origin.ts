import type { CoreInstallation } from "@oac/agents-client";

import { isValidDirectCoreBaseUrl } from "../../lib/connection";

/**
 * The value itself when it is an origin the node commands may use directly: no
 * path, query, fragment or credentials; a single trailing slash is dropped. It
 * is kept as written, not normalized, so an explicit port such as :443 stays
 * exactly as Core reports it. HTTPS always qualifies; plain HTTP only on a
 * loopback host, or on any host when `allowInsecure` is set (a development
 * installation with `allow_insecure_origin`).
 */
function origin(value: string, allowInsecure: boolean): string | null {
  const candidate = value.trim().replace(/\/$/, "");
  const pattern = allowInsecure ? /^https?:\/\/[^/?#\\\s@]+$/i : /^https:\/\/[^/?#\\\s@]+$/i;
  return pattern.test(candidate) && isValidDirectCoreBaseUrl(candidate, allowInsecure) ? candidate : null;
}

/** The HTTPS origin form, the default the node commands require. */
export function httpsOrigin(value: string): string | null {
  return origin(value, false);
}

/**
 * Where the node commands download the installer, and the `--source-url` they
 * pass it: the installation's public URL, whose reverse proxy sends
 * `/node-install/*` to this console. Unlike the browser's address, it is the
 * same from every machine. Null when other machines can't use it: loopback
 * (`local_only`), missing, or not an HTTPS origin unless `allowInsecure` is set.
 */
export function nodeSourceUrl(installation: Pick<CoreInstallation, "public_url" | "local_only">, allowInsecure = false): string | null {
  if (installation.local_only || !installation.public_url) return null;
  return origin(installation.public_url, allowInsecure);
}

/**
 * Whether this installation allows a non-loopback plain-HTTP origin. Core's
 * settings snapshot (`allow_insecure_origin`, from `OAC_ALLOW_INSECURE_ORIGIN`)
 * is the only source: the console never reads that environment variable or a
 * second field. Absent, the switch is off.
 */
export function allowsInsecureOrigin(installation: Pick<CoreInstallation, "configuration"> | undefined): boolean {
  return installation?.configuration?.settings.find((entry) => entry.key === "allow_insecure_origin")?.value === true;
}
