

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { configDir } from "./idaptpaths";

const MAX_PER_RESOURCE = 200;

export interface CachedId {
  readonly id: string;

  readonly label?: string;
}

type CacheFile = Record<string, CachedId[]>;

function cachePath(): string {
  return join(configDir(), "id-cache.json");
}

function readCache(): CacheFile {
  try {
    const raw = readFileSync(cachePath(), "utf-8");
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return {};
    return parsed as CacheFile;
  } catch {

    return {};
  }
}

export function idsFromRows(rows: readonly unknown[]): CachedId[] {
  const out: CachedId[] = [];
  for (const row of rows) {
    if (!row || typeof row !== "object") continue;
    const rec = row as Record<string, unknown>;
    const id = rec.id ?? rec.resource_id;
    if (typeof id !== "string" || id.length === 0) continue;
    const label = rec.name ?? rec.title ?? rec.slug ?? rec.key;
    out.push(
      typeof label === "string" && label.length > 0 ? { id, label } : { id },
    );
  }
  return out;
}

export function rememberIds(resource: string, ids: readonly CachedId[]): void {
  if (ids.length === 0) return;
  try {
    const cache = readCache();
    cache[resource] = ids.slice(0, MAX_PER_RESOURCE);
    mkdirSync(configDir(), { recursive: true });
    writeFileSync(cachePath(), JSON.stringify(cache), { mode: 0o600 });
  } catch {

  }
}

export function cachedIds(resource: string): CachedId[] {
  const entry = readCache()[resource];
  return Array.isArray(entry) ? entry : [];
}
