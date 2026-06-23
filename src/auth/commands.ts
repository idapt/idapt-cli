
import { readFileSync } from "node:fs";
import { createFetchTransport } from "../transport";
import { USER_AGENT } from "../version";
import { canOpenBrowser, openBrowser } from "./browser";
import {
  type Credentials,
  clearCredentials,
  loadCredentials,
  saveCredentials,
} from "./credentials";
import { loginAuthCode, loginDevice } from "./oauth";
import { resolveCredential } from "./resolve";

export const AUTH_HINT = `Not signed in.

Sign in in your browser (recommended):
  idapt login

On a headless machine / over SSH (device code):
  idapt login --device

Or paste an API key (get one from https://idapt.app/settings/api-keys):
  printf '%s' "$KEY" | idapt login --api-key-stdin
  export IDAPT_API_KEY=<key>        # or via env (CI / sudo)`;

export interface AuthCtx {

  readonly out: (s: string) => void;

  readonly print: (s: string) => void;
  readonly env: Record<string, string | undefined>;
  readonly isTty: boolean;
  readonly baseUrl: string;
  readonly apiKeyFlag?: string;

  readonly readStdin?: () => string;
}

export interface LoginOptions {
  readonly device?: boolean;
  readonly web?: boolean;
  readonly apiKeyStdin?: boolean;
}

async function fetchIdentity(
  baseUrl: string,
  token: string,
): Promise<{ email?: string; name?: string } | null> {
  try {
    const t = createFetchTransport({ baseUrl, token, userAgent: USER_AGENT });
    const res = await t.request({ method: "GET", path: "/api/v1/me" });
    if (res.status !== 200) return null;
    const data = ((res.json as { data?: Record<string, unknown> })?.data ??
      {}) as Record<string, unknown>;
    const email = (data.userEmail ?? data.email) as string | undefined;
    const name = (data.userName ?? data.name) as string | undefined;
    return { email: email ?? undefined, name: name ?? undefined };
  } catch {
    return null;
  }
}

async function announceSignedIn(ctx: AuthCtx, token: string): Promise<void> {
  const id = await fetchIdentity(ctx.baseUrl, token);
  if (id?.email) {
    ctx.out(`\n✓ Signed in as ${id.email}${id.name ? ` (${id.name})` : ""}.`);
  } else {
    ctx.out("\n✓ Signed in.");
  }
}

export async function runLogin(
  ctx: AuthCtx,
  opts: LoginOptions,
): Promise<number> {

  if (opts.apiKeyStdin) {
    const key = (ctx.readStdin ?? (() => readFileSync(0, "utf8")))().trim();
    if (!key) {
      ctx.out("idapt login: no API key on stdin.");
      return 1;
    }
    saveCredentials({ apiKey: key });
    await announceSignedIn(ctx, key);
    return 0;
  }

  const useDevice =
    opts.device || (!opts.web && !canOpenBrowser(ctx.env, ctx.isTty));

  const controller = new AbortController();
  const onSigint = () => controller.abort();
  process.once("SIGINT", onSigint);
  try {
    if (useDevice) {
      const sessionToken = await loginDevice({
        out: ctx.out,
        baseUrl: ctx.baseUrl,
        userAgent: USER_AGENT,
        signal: controller.signal,
      });
      saveCredentials({ apiKey: sessionToken });
      await announceSignedIn(ctx, sessionToken);
      return 0;
    }
    const tokens = await loginAuthCode({
      out: ctx.out,
      baseUrl: ctx.baseUrl,
      userAgent: USER_AGENT,
      open: openBrowser,
      signal: controller.signal,
    });
    const creds: Credentials = {
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + (tokens.expiresIn || 900),
    };
    saveCredentials(creds);
    await announceSignedIn(ctx, tokens.accessToken);
    return 0;
  } catch (err) {
    ctx.out(`\n✗ ${(err as Error).message}`);
    return 1;
  } finally {
    process.removeListener("SIGINT", onSigint);
  }
}

export function runLogout(ctx: AuthCtx): number {
  clearCredentials();
  ctx.out("Signed out — cleared the stored credential.");
  return 0;
}

export async function runStatus(ctx: AuthCtx): Promise<number> {
  const resolved = await resolveCredential({
    apiKeyFlag: ctx.apiKeyFlag,
    env: ctx.env,
    baseUrl: ctx.baseUrl,
    userAgent: USER_AGENT,
  }).catch(() => null);

  if (!resolved) {
    ctx.print(AUTH_HINT);
    return 1;
  }

  const lines: string[] = [
    `API URL:  ${ctx.baseUrl}`,
    `Source:   ${resolved.source}`,
    `Kind:     ${resolved.kind}`,
  ];
  if (resolved.kind === "oauth") {
    const creds = loadCredentials();
    if (creds.expiresAt) {
      const secs = creds.expiresAt - Math.floor(Date.now() / 1000);
      lines.push(
        `Token:    ${secs > 0 ? `valid (${Math.max(0, Math.round(secs / 60))} min left)` : "expired (auto-refreshes on next call)"}`,
      );
    }
  }

  const id = await fetchIdentity(ctx.baseUrl, resolved.token);
  if (!id) {
    ctx.print(lines.join("\n"));
    ctx.out("\n✗ Credential is invalid or expired — run `idapt login`.");
    return 1;
  }
  lines.unshift(
    `Signed in as ${id.email ?? "(unknown)"}${id.name ? ` (${id.name})` : ""}`,
    "",
  );
  ctx.print(lines.join("\n"));
  return 0;
}
