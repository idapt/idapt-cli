
import {
  chmodSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { configDir } from "../idaptpaths";

export interface Credentials {
  apiKey?: string;
  accessToken?: string;
  refreshToken?: string;

  expiresAt?: number;

  defaultWorkspaceId?: string;

  defaultWorkspaceSlug?: string;
}

export function credentialsPath(): string {
  return join(configDir(), "cli-auth.json");
}

export function loadCredentials(path = credentialsPath()): Credentials {
  let raw: string;
  try {
    raw = readFileSync(path, "utf8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") return {};
    throw err;
  }
  if (!raw.trim()) return {};
  return JSON.parse(raw) as Credentials;
}

export function saveCredentials(
  creds: Credentials,
  path = credentialsPath(),
): void {
  mkdirSync(dirname(path), { recursive: true, mode: 0o700 });
  writeFileSync(path, `${JSON.stringify(creds, null, 2)}\n`, { mode: 0o600 });
  chmodSync(path, 0o600);
}

export function clearCredentials(path = credentialsPath()): void {
  try {
    rmSync(path);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") throw err;
  }
}

export function hasOAuth(creds: Credentials): boolean {
  return Boolean(creds.refreshToken);
}
