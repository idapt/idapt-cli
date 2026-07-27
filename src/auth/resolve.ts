
import { loadActiveCredentials, saveActiveCredentials } from "./contexts";
import { type Credentials, hasOAuth } from "./credentials";
import { refreshAccessToken } from "./oauth";

const SKEW_SECONDS = 60;

export type CredentialSource = "flag" | "env" | "file";
export type CredentialKind = "api-key" | "oauth";

export interface ResolvedCredential {
  readonly token: string;
  readonly source: CredentialSource;
  readonly kind: CredentialKind;
}

export interface ResolveOptions {
  readonly apiKeyFlag?: string;
  readonly env: Record<string, string | undefined>;
  readonly baseUrl: string;
  readonly userAgent: string;
}

export async function resolveCredential(
  opts: ResolveOptions,
): Promise<ResolvedCredential | null> {
  if (opts.apiKeyFlag) {
    return { token: opts.apiKeyFlag, source: "flag", kind: "api-key" };
  }
  const envKey = opts.env.IDAPT_API_KEY;
  if (envKey) return { token: envKey, source: "env", kind: "api-key" };

  const creds = loadActiveCredentials();
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
      accessToken: tok.accessToken,
      refreshToken: tok.refreshToken || creds.refreshToken,
      expiresAt: now + (tok.expiresIn || 900),
    };
    saveActiveCredentials(updated);
    return { token: tok.accessToken, source: "file", kind: "oauth" };
  }

  if (creds.apiKey)
    return { token: creds.apiKey, source: "file", kind: "api-key" };
  return null;
}
