
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { cacheDir } from "../idaptpaths";
import { VERSION } from "../version";

export const PACKAGE_NAME = "@idapt/cli";
const REGISTRY = "https://registry.npmjs.org";
const DAY_MS = 24 * 60 * 60 * 1000;

export interface NotifyIo {
  readonly stderr: (s: string) => void;
  readonly env: Record<string, string | undefined>;
  readonly isTty: boolean;
}

interface CheckState {
  lastCheckAt?: number;
  lastShownAt?: number;
  latest?: string;
}

function statePath(): string {
  return join(cacheDir(), "cli-update-check.json");
}

function loadState(): CheckState {
  try {
    return JSON.parse(readFileSync(statePath(), "utf8")) as CheckState;
  } catch {
    return {};
  }
}

function saveState(s: CheckState): void {
  try {
    mkdirSync(cacheDir(), { recursive: true });
    writeFileSync(statePath(), JSON.stringify(s));
  } catch {

  }
}

export function isNewer(latest: string, current: string): boolean {
  const norm = (v: string) =>
    v
      .replace(/^v/, "")
      .split("-")[0]
      .split(".")
      .map((n) => Number.parseInt(n, 10) || 0);
  const a = norm(latest);
  const b = norm(current);
  for (let i = 0; i < 3; i++) {
    const x = a[i] ?? 0;
    const y = b[i] ?? 0;
    if (x > y) return true;
    if (x < y) return false;
  }
  return false;
}

export async function fetchDistTag(
  tag = "latest",
  timeoutMs = 1500,
): Promise<string | null> {
  try {
    const res = await fetch(`${REGISTRY}/${encodeURIComponent(PACKAGE_NAME)}`, {
      headers: { Accept: "application/vnd.npm.install-v1+json" },
      signal: AbortSignal.timeout(timeoutMs),
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { "dist-tags"?: Record<string, string> };
    return body["dist-tags"]?.[tag] ?? null;
  } catch {
    return null;
  }
}

function disabled(env: Record<string, string | undefined>): boolean {
  return Boolean(env.CI || env.NO_UPDATE_NOTIFIER || env.IDAPT_NO_UPDATE_NUDGE);
}

export async function maybeNotify(io: NotifyIo): Promise<void> {
  try {
    if (!io.isTty || disabled(io.env)) return;
    if (VERSION.includes("-dev")) return;
    const now = Date.now();
    const state = loadState();
    if (!state.lastCheckAt || now - state.lastCheckAt > DAY_MS) {
      const latest = await fetchDistTag("latest");
      state.lastCheckAt = now;
      if (latest) state.latest = latest;
      saveState(state);
    }
    if (!state.latest || !isNewer(state.latest, VERSION)) return;
    if (state.lastShownAt && now - state.lastShownAt < DAY_MS) return;
    state.lastShownAt = now;
    saveState(state);
    io.stderr(
      `\n  ◆ Update available: idapt ${VERSION} → ${state.latest}. Run \`idapt upgrade\`.\n`,
    );
  } catch {

  }
}
