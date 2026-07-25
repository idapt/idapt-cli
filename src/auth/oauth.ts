
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { generatePkce, randomState } from "./pkce";

const AUTHORIZE_PATH = "/api/auth/oauth2/authorize";
const TOKEN_PATH = "/api/auth/oauth2/token";
const DEVICE_CODE_PATH = "/api/auth/device/code";
const DEVICE_TOKEN_PATH = "/api/auth/device/token";
const DEVICE_GRANT = "urn:ietf:params:oauth:grant-type:device_code";

export const CLI_CLIENT_ID = "idapt-cli";
const SCOPE = "openid profile email offline_access";

const RESOURCE = "idapt-api";
const LOGIN_TIMEOUT_MS = 5 * 60_000;

export class AuthError extends Error {}
export const ERR_ACCESS_DENIED = "the login request was denied";
export const ERR_STATE_MISMATCH = "login state mismatch — please try again";
export const ERR_TIMED_OUT = "timed out waiting for browser sign-in";
export const ERR_SESSION_EXPIRED =
  "your session has expired — run `idapt login` again";

export interface OAuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface FlowIo {

  readonly out: (s: string) => void;
  readonly baseUrl: string;
  readonly userAgent: string;

  readonly open?: (url: string) => boolean;

  readonly signal?: AbortSignal;
}

function trimRight(s: string): string {
  return s.replace(/\/+$/, "");
}

function originOf(baseUrl: string): string {
  const u = new URL(baseUrl);
  return `${u.protocol}//${u.host}`;
}

async function postToken(
  baseUrl: string,
  form: Record<string, string>,
  userAgent: string,
): Promise<OAuthTokens> {
  const base = trimRight(baseUrl);
  const res = await fetch(base + TOKEN_PATH, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
      Origin: originOf(base),
      "User-Agent": userAgent,
    },
    body: new URLSearchParams(form).toString(),
  });
  const body = (await res.json().catch(() => ({}))) as {
    access_token?: string;
    refresh_token?: string;
    expires_in?: number;
    error?: string;
    error_description?: string;
  };
  if (!res.ok || !body.access_token) {
    if (body.error === "invalid_grant")
      throw new AuthError(ERR_SESSION_EXPIRED);
    const msg = body.error_description || body.error || `HTTP ${res.status}`;
    throw new AuthError(`token request failed (${res.status}): ${msg}`);
  }
  return {
    accessToken: body.access_token,
    refreshToken: body.refresh_token ?? "",
    expiresIn: body.expires_in ?? 0,
  };
}

const CALLBACK_OK = `<!doctype html><html><head><meta charset="utf-8"><title>Signed in</title></head><body style="font-family:system-ui;text-align:center;padding-top:4rem"><h1>You're signed in</h1><p>Return to your terminal — you can close this tab.</p></body></html>`;
const CALLBACK_ERR = `<!doctype html><html><head><meta charset="utf-8"><title>Sign-in failed</title></head><body style="font-family:system-ui;text-align:center;padding-top:4rem"><h1>Sign-in failed</h1><p>Return to your terminal and run <code>idapt login</code> again.</p></body></html>`;

export function loginAuthCode(io: FlowIo): Promise<OAuthTokens> {
  const base = trimRight(io.baseUrl);
  const { verifier, challenge } = generatePkce();
  const state = randomState();

  return new Promise<OAuthTokens>((resolve, reject) => {
    let settled = false;
    const finish = (fn: () => void) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      io.signal?.removeEventListener("abort", onAbort);
      server.close();
      fn();
    };
    const onAbort = () =>
      finish(() => reject(new AuthError("sign-in cancelled")));

    const server = createServer((req, res) => {
      const url = new URL(req.url ?? "/", "http://127.0.0.1");
      if (url.pathname !== "/callback") {
        res.writeHead(404).end();
        return;
      }
      const q = url.searchParams;
      const err = q.get("error");
      if (err) {
        res.writeHead(400, { "Content-Type": "text/html" }).end(CALLBACK_ERR);
        finish(() =>
          reject(
            new AuthError(
              err === "access_denied"
                ? ERR_ACCESS_DENIED
                : `sign-in failed: ${err}`,
            ),
          ),
        );
        return;
      }
      if (q.get("state") !== state) {
        res.writeHead(400, { "Content-Type": "text/html" }).end(CALLBACK_ERR);
        finish(() => reject(new AuthError(ERR_STATE_MISMATCH)));
        return;
      }
      const code = q.get("code");
      if (!code) {
        res.writeHead(400, { "Content-Type": "text/html" }).end(CALLBACK_ERR);
        finish(() =>
          reject(
            new AuthError("the sign-in response had no authorization code"),
          ),
        );
        return;
      }
      res.writeHead(200, { "Content-Type": "text/html" }).end(CALLBACK_OK);
      const redirectUri = `http://127.0.0.1:${port}/callback`;
      postToken(
        base,
        {
          grant_type: "authorization_code",
          client_id: CLI_CLIENT_ID,
          code,
          code_verifier: verifier,
          redirect_uri: redirectUri,
          resource: RESOURCE,
        },
        io.userAgent,
      ).then(
        (tok) => finish(() => resolve(tok)),
        (e) => finish(() => reject(e)),
      );
    });

    let port = 0;
    const timer = setTimeout(
      () => finish(() => reject(new AuthError(ERR_TIMED_OUT))),
      LOGIN_TIMEOUT_MS,
    );
    io.signal?.addEventListener("abort", onAbort);

    server.on("error", (e) => finish(() => reject(e)));
    server.listen(0, "127.0.0.1", () => {
      port = (server.address() as AddressInfo).port;
      const redirectUri = `http://127.0.0.1:${port}/callback`;
      const authzUrl =
        `${base}${AUTHORIZE_PATH}?` +
        new URLSearchParams({
          response_type: "code",
          client_id: CLI_CLIENT_ID,
          redirect_uri: redirectUri,
          scope: SCOPE,
          state,
          code_challenge: challenge,
          code_challenge_method: "S256",
        }).toString();

      io.out("");
      const opened = io.open?.(authzUrl) ?? false;
      io.out(
        opened
          ? `Opening your browser to sign in…\n\nIf it doesn't open, visit:\n\n    ${authzUrl}\n`
          : `To finish signing in, open this URL in your browser:\n\n    ${authzUrl}\n`,
      );
      io.out("Waiting for sign-in… (Ctrl-C to cancel)");
    });
  });
}

interface DeviceCode {
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete?: string;
  expires_in: number;
  interval: number;
}

const sleep = (ms: number, signal?: AbortSignal) =>
  new Promise<void>((resolve, reject) => {
    const t = setTimeout(resolve, ms);
    signal?.addEventListener("abort", () => {
      clearTimeout(t);
      reject(new AuthError("sign-in cancelled"));
    });
  });

export async function loginDevice(io: FlowIo): Promise<string> {
  const base = trimRight(io.baseUrl);
  const codeRes = await fetch(base + DEVICE_CODE_PATH, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Origin: originOf(base),
      "User-Agent": io.userAgent,
    },
    body: JSON.stringify({ client_id: CLI_CLIENT_ID }),
  });
  if (!codeRes.ok) {
    throw new AuthError(
      `could not start device login (HTTP ${codeRes.status})`,
    );
  }
  const code = (await codeRes.json()) as DeviceCode;
  if (!code.device_code || !code.user_code) {
    throw new AuthError(
      "the server returned an incomplete login-code response",
    );
  }

  io.out("");
  if (code.verification_uri_complete) {
    io.out(
      `To finish signing in, open this URL in your browser:\n\n    ${code.verification_uri_complete}\n\n(it confirms code ${code.user_code})`,
    );
  } else {
    io.out(
      `To finish signing in, open:\n\n    ${code.verification_uri}\n\nand enter the code:  ${code.user_code}`,
    );
  }
  io.out("\nWaiting for approval… (Ctrl-C to cancel)");

  let interval = code.interval > 0 ? code.interval : 5;
  const deadline =
    Date.now() + (code.expires_in > 0 ? code.expires_in : 900) * 1000;
  for (;;) {
    await sleep(interval * 1000, io.signal);
    if (Date.now() > deadline) {
      throw new AuthError("the login code expired before it was approved");
    }
    const res = await fetch(base + DEVICE_TOKEN_PATH, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Origin: originOf(base),
        "User-Agent": io.userAgent,
      },
      body: JSON.stringify({
        grant_type: DEVICE_GRANT,
        device_code: code.device_code,
        client_id: CLI_CLIENT_ID,
      }),
    });
    const body = (await res.json().catch(() => ({}))) as {
      access_token?: string;
      error?: string;
    };
    if (res.ok && body.access_token) return body.access_token;
    switch (body.error) {
      case "authorization_pending":
        break;
      case "slow_down":
        interval += 5;
        break;
      case "expired_token":
        throw new AuthError("the login code expired before it was approved");
      case "access_denied":
        throw new AuthError(ERR_ACCESS_DENIED);
      default:
        throw new AuthError(`device login failed (HTTP ${res.status})`);
    }
  }
}

export function refreshAccessToken(
  baseUrl: string,
  refreshToken: string,
  userAgent: string,
): Promise<OAuthTokens> {
  return postToken(
    baseUrl,
    {
      grant_type: "refresh_token",
      client_id: CLI_CLIENT_ID,
      refresh_token: refreshToken,
      resource: RESOURCE,
    },
    userAgent,
  );
}
