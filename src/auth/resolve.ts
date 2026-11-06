
import { loadActiveCredentials, saveActiveCredentials } from "./contexts";
import { type Credentials, hasOAuth } from "./credentials";
import { refreshAccessToken } from "./oauth";

const SKEW_SECONDS = 60;

export class CredentialOriginError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "CredentialOriginError";
  }
}

function originOf(url: string): string | null {
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

function isLoopbackHost(hostname: string): boolean {
  const host = hostname.replace(/^\[|]$/g, "").toLowerCase();
  return (
    host === "localhost" ||
    host === "::1" ||
    host === "0.0.0.0" ||
    /^127(\.\d{1,3}){3}$/.test(host) ||
    host.endsWith(".localhost")
  );
}

export function storedCredentialRefusal(
  storedApiUrl: string | undefined,
  baseUrl: string,
): string | null {
  let parsed: URL;
  try {
    parsed = new URL(baseUrl);
  } catch {
    return `idapt: "${baseUrl}" is not a valid API URL.`;
  }

  if (parsed.protocol !== "https:" && !isLoopbackHost(parsed.hostname)) {
    return (
      `idapt: refusing to send your stored credential to ${parsed.origin} over ${parsed.protocol.replace(":", "")}.\n` +
      "  Use an https URL, or pass the key explicitly with --api-key for a plaintext host."
    );
  }

  const bound = storedApiUrl ? originOf(storedApiUrl) : null;
  if (bound && bound !== parsed.origin) {
    return (
      `idapt: your stored credential was issued for ${bound}, not ${parsed.origin}.\n` +
      `  Sign in against the new origin (\`idapt login --api-url ${parsed.origin}\`),\n` +
      "  or pass a key for it with --api-key."
    );
  }
  return null;
}

export type CredentialSource = "flag" | "env" | "file";
export type CredentialKind = "api-key" | "oauth";

export interface ResolvedCredential {
  readonly token: string;
  readonly source: CredentialSource;
  readonly kind: CredentialKind;

  readonly isWorkspaceScoped?: boolean;
}

export interface ResolveOptions {
  readonly apiKeyFlag?: string;
  readonly env: Record<string, string | undefined>;
  readonly baseUrl: string;
  readonly userAgent: string;

  readonly contextFlag?: string;
}

function isWorkspaceScopedKey(token: string): boolean {
  return /^pk_/.test(token);
}

export async function resolveCredential(
  opts: ResolveOptions,
): Promise<ResolvedCredential | null> {
  if (opts.apiKeyFlag) {
    return {
      token: opts.apiKeyFlag,
      source: "flag",
      kind: "api-key",
      isWorkspaceScoped: isWorkspaceScopedKey(opts.apiKeyFlag),
    };
  }
  const envKey = opts.env.IDAPT_API_KEY;
  if (envKey) {
    return {
      token: envKey,
      source: "env",
      kind: "api-key",
      isWorkspaceScoped: isWorkspaceScopedKey(envKey),
    };
  }

  const contextOpts = {
    ...(opts.contextFlag ? { flag: opts.contextFlag } : {}),
    env: opts.env,
  };
  const creds = loadActiveCredentials(contextOpts);

  if (creds.accessToken || creds.refreshToken || creds.apiKey) {
    const refusal = storedCredentialRefusal(creds.apiUrl, opts.baseUrl);
    if (refusal) throw new CredentialOriginError(refusal);
  }
  if (hasOAuth(creds)) {
    const now = Math.floor(Date.now() / 1000);
    if (
      creds.accessToken &&
      creds.expiresAt &&
      creds.expiresAt - now > SKEW_SECONDS
    ) {
      return { token: creds.accessToken, source: "file", kind: "oauth" };
    }
    const tok = await refreshAccessToken(
      opts.baseUrl,

      creds.refreshToken as string,
      opts.userAgent,
    );
    const updated: Credentials = {
      ...creds,
      accessToken: tok.accessToken,
      refreshToken: tok.refreshToken || creds.refreshToken,
      expiresAt: now + (tok.expiresIn || 900),
    };
    saveActiveCredentials(updated, contextOpts);
    return { token: tok.accessToken, source: "file", kind: "oauth" };
  }

  if (creds.apiKey) {
    return {
      token: creds.apiKey,
      source: "file",
      kind: "api-key",
      isWorkspaceScoped: isWorkspaceScopedKey(creds.apiKey),
    };
  }
  return null;
}
