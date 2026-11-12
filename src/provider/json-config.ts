

export class JsonEditError extends Error {}

export interface JsonEditResult {
  text: string;
  changed: boolean;
}

type Json = Record<string, unknown>;

function isPlainObject(value: unknown): value is Json {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function deepMerge(base: Json, patch: Json): Json {
  const out: Json = { ...base };
  for (const [key, value] of Object.entries(patch)) {
    const existing = out[key];
    if (isPlainObject(existing) && isPlainObject(value)) {
      out[key] = deepMerge(existing, value);
    } else {
      out[key] = value;
    }
  }
  return out;
}

export function mergeJsonConfig(existing: string, patch: Json): JsonEditResult {
  const trimmed = existing.trim();
  if (trimmed === "") {
    return { text: `${JSON.stringify(patch, null, 2)}\n`, changed: true };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (error) {
    throw new JsonEditError(
      `Existing config is not valid JSON: ${(error as Error).message}`,
    );
  }
  if (!isPlainObject(parsed)) {
    throw new JsonEditError("Existing config JSON is not an object");
  }

  const merged = deepMerge(parsed, patch);
  const nextText = `${JSON.stringify(merged, null, 2)}\n`;
  return { text: nextText, changed: nextText !== existing };
}

export function readJsonPath(text: string, path: string[]): unknown {
  if (text.trim() === "") return undefined;
  let current: unknown;
  try {
    current = JSON.parse(text);
  } catch {
    return undefined;
  }
  for (const segment of path) {
    if (!isPlainObject(current)) return undefined;
    current = current[segment];
  }
  return current;
}
