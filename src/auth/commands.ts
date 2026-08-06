

import { readFileSync } from "node:fs";
import { buildAbsoluteAppUrl } from "@shared/router/build-path";
import { symbols } from "../text";
import { createFetchTransport } from "../transport";
import { USER_AGENT } from "../version";
import { effectiveWorkspace, originLabel } from "../workspace-ref";
import { canOpenBrowser, openBrowser } from "./browser";
import {
  clearActiveCredentials,
  loadActiveCredentials,
  loadContexts,
  saveActiveCredentials,
} from "./contexts";
import { type Credentials, credentialsPath } from "./credentials";
import { loginAuthCode, loginDevice } from "./oauth";
import { resolveCredential } from "./resolve";

const API_KEYS_URL = buildAbsoluteAppUrl("https://idapt.app", {
  type: "settings",
  section: "developer",
});

export function authHint(
  env: Record<string, string | undefined> = process.env,
): string {
  const powershell = Boolean(env.PSModulePath) || process.platform === "win32";
  const paste = powershell
    ? [
        "  $env:IDAPT_API_KEY = '<key>'",
        "  # or, to store it: $env:KEY | idapt login --api-key-stdin",
      ]
    : [
        "  export IDAPT_API_KEY=<key>",
        "  # or, to store it: printf '%s' \"$KEY\" | idapt login --api-key-stdin",
      ];
  return [
    "Not signed in.",
    "",
    "Sign in in your browser (recommended):",
    "  idapt login",
    "",
    "On a headless machine or over SSH (device code):",
    "  idapt login --device",
    "",
    `Or use an API key from ${API_KEYS_URL}:`,
    ...paste,

    "",
  ].join("\n");
}

export const AUTH_HINT = authHint();

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

  readonly context?: string;
}

async function fetchIdentity(
  baseUrl: string,
  token: string,
): Promise<{
  email?: string;
  name?: string;
  enabledFeatures?: string[];
} | null> {
  try {
    const t = createFetchTransport({ baseUrl, token, userAgent: USER_AGENT });
    const res = await t.request({ method: "GET", path: "/api/v1/me" });
    if (res.status !== 200) return null;
    const data = ((res.json as { data?: Record<string, unknown> })?.data ??
      {}) as Record<string, unknown>;
    const email = (data.userEmail ?? data.email) as string | undefined;
    const name = (data.userName ?? data.name) as string | undefined;
    const features = data.enabled_features;
    return {
      email: email ?? undefined,
      name: name ?? undefined,
      ...(Array.isArray(features)
        ? { enabledFeatures: features as string[] }
        : {}),
    };
  } catch {
    return null;
  }
}

async function announceSignedIn(
  ctx: AuthCtx,
  token: string,
  contextName?: string,
): Promise<void> {
  const id = await fetchIdentity(ctx.baseUrl, token);

  if (id?.enabledFeatures) {
    const contextOpts = contextName ? { flag: contextName } : undefined;
    saveActiveCredentials(
      {
        ...loadActiveCredentials(contextOpts),
        enabledFeatures: id.enabledFeatures,
      },
      contextOpts,
    );
  }
  const who = id?.email ? `as ${id.email}${displayName(id.name)}` : "";
  const where = contextName ? ` (account "${contextName}")` : "";
  ctx.out(
    `\n${symbols(ctx.env).tick} Signed in ${who}${where}.`.replace("  ", " "),
  );

  if (ctx.env.IDAPT_API_KEY) {
    ctx.out(
      "\nNote: IDAPT_API_KEY is set in your environment and takes precedence over this sign-in.\n" +
        "Unset it to use the account you just signed in as.",
    );
  }

  ctx.out(
    "\nNext: `idapt workspace use <workspace>` to pick where commands run, or `idapt help`.\n",
  );
}

function previousIdentity(contextName?: string): string | null {
  const store = loadContexts();
  const name = contextName ?? store.current;
  const existing = store.contexts[name];
  if (!existing) return null;
  return existing.accessToken || existing.apiKey ? name : null;
}

function ctxOpts(opts: LoginOptions): { flag?: string } {
  return opts.context ? { flag: opts.context } : {};
}

export async function runLogin(
  ctx: AuthCtx,
  opts: LoginOptions,
): Promise<number> {

  const replacing = previousIdentity(opts.context);
  if (replacing && !opts.context) {
    ctx.out(`Replacing the credential stored for account "${replacing}".\n`);
  }

  if (opts.apiKeyStdin) {
    const key = (ctx.readStdin ?? (() => readFileSync(0, "utf8")))().trim();
    if (!key) {
      ctx.out("idapt login: no API key on stdin.");
      return 1;
    }
    saveActiveCredentials({ apiKey: key }, ctxOpts(opts));
    await announceSignedIn(ctx, key, opts.context);
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
      saveActiveCredentials({ apiKey: sessionToken }, ctxOpts(opts));
      await announceSignedIn(ctx, sessionToken, opts.context);
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
    saveActiveCredentials(creds, ctxOpts(opts));
    await announceSignedIn(ctx, tokens.accessToken, opts.context);
    return 0;
  } catch (err) {
    ctx.out(`\n✗ ${(err as Error).message}`);
    return 1;
  } finally {
    process.removeListener("SIGINT", onSigint);
  }
}

export function runLogout(ctx: AuthCtx): number {
  clearActiveCredentials();
  ctx.out("Signed out. The stored credential was removed.\n");
  return 0;
}

export async function runStatus(
  ctx: AuthCtx,
  contextFlag?: string,

  asJson = false,
): Promise<number> {
  const resolved = await resolveCredential({
    apiKeyFlag: ctx.apiKeyFlag,
    env: ctx.env,
    baseUrl: ctx.baseUrl,
    userAgent: USER_AGENT,
    ...(contextFlag ? { contextFlag } : {}),
  }).catch(() => null);

  if (!resolved) {
    ctx.print(authHint(ctx.env));
    return 1;
  }

  const store = loadContexts();
  const creds = loadActiveCredentials(
    contextFlag ? { flag: contextFlag } : undefined,
  );

  const effective = effectiveWorkspace({
    env: ctx.env,
    pin: creds.defaultWorkspaceSlug ?? creds.defaultWorkspaceId,
  });
  const workspaceLine = effective
    ? `${effective.ref} (from ${originLabel(effective.origin)})`
    : "your default workspace (nothing pinned)";
  const lines: string[] = [
    `Account:   ${contextFlag ?? store.current}`,
    `API URL:   ${ctx.baseUrl}`,
    `Source:    ${describeSource(resolved.source)}`,
    `Kind:      ${resolved.kind}${resolved.isWorkspaceScoped ? " (pinned to one workspace)" : ""}`,
    `Workspace: ${workspaceLine}`,
    `Agent:     ${creds.defaultAgentSlug ?? creds.defaultAgentId ?? "you (no agent selected)"}`,
    `Config:    ${credentialsPath()}`,
  ];
  if (resolved.kind === "oauth") {
    if (creds.expiresAt) {
      const secs = creds.expiresAt - Math.floor(Date.now() / 1000);
      lines.push(
        `Token:    ${secs > 0 ? `valid (${Math.max(0, Math.round(secs / 60))} min left)` : "expired (auto-refreshes on next call)"}`,
      );
    }
  }

  const id = await fetchIdentity(ctx.baseUrl, resolved.token);

  if (asJson) {
    ctx.print(
      `${JSON.stringify(
        {
          signed_in: Boolean(id),
          email: id?.email ?? null,
          name: id?.name ?? null,
          account: contextFlag ?? store.current,
          api_url: ctx.baseUrl,
          credential_source: resolved.source,
          credential_kind: resolved.kind,
          workspace_scoped_credential: resolved.isWorkspaceScoped ?? false,
          workspace: effective?.ref ?? null,
          workspace_origin: effective?.origin ?? "default",
          agent: creds.defaultAgentSlug ?? creds.defaultAgentId ?? null,
          config_path: credentialsPath(),
        },
        null,
        2,
      )}\n`,
    );
    return id ? 0 : 1;
  }

  if (!id) {
    ctx.print(lines.join("\n"));
    ctx.out("\n✗ Credential is invalid or expired - run `idapt login`.");
    return 1;
  }
  lines.unshift(
    `Signed in as ${id.email ?? "(unknown)"}${displayName(id.name)}`,
    "",
  );

  ctx.print(`${lines.join("\n")}\n`);
  return 0;
}

function displayName(name: string | null | undefined): string {
  const trimmed = (name ?? "").trim();
  if (!trimmed || trimmed === "-") return "";
  return ` (${trimmed})`;
}

function describeSource(source: "flag" | "env" | "file"): string {
  switch (source) {
    case "flag":
      return "--api-key on this command";
    case "env":
      return "IDAPT_API_KEY (environment)";
    case "file":
      return "stored sign-in";
  }
}
