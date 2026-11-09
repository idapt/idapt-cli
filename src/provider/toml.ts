

export class TomlEditError extends Error {}

export type TomlValue = string | number | boolean;

export interface TomlEditResult {
  text: string;
  changed: boolean;
}

interface Block {

  path: string[] | null;

  header: string | null;
  lines: string[];
}

const HEADER_RE = /^(\[\[?[^\]]*\]\]?)(?:\s*#.*)?$/;
const KEY_RE = /^(\s*)("?[A-Za-z0-9_.-]+"?)\s*=\s*([^#]*?)(\s*#.*)?$/;

export function assertParseable(text: string): void {

  const withoutMultiline = text.replace(/"""[\s\S]*?"""/g, "");
  const lines = withoutMultiline.split("\n");
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("[")) {
      if (!HEADER_RE.test(trimmed)) {
        throw new TomlEditError(`Malformed TOML table header: ${trimmed}`);
      }
      continue;
    }
    if (trimmed === "" || trimmed.startsWith("#")) continue;

    const unescaped = trimmed.replace(/\\"/g, "").split('"').length - 1;
    if (unescaped % 2 !== 0) {
      throw new TomlEditError(`Unbalanced quote in line: ${trimmed}`);
    }
  }
}

function splitBlocks(text: string): Block[] {
  const lines = text.split("\n");
  const blocks: Block[] = [];
  let current: Block = { path: null, header: null, lines: [] };
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("[")) {
      const match = HEADER_RE.exec(trimmed);
      if (!match)
        throw new TomlEditError(`Malformed TOML table header: ${trimmed}`);
      blocks.push(current);
      const headerContent = (match[1] as string)
        .replace(/^\[\[?/, "")
        .replace(/\]\]?$/, "");
      const path = headerContent
        .split(".")
        .map((part) => part.trim().replace(/^["']|["']$/g, ""));
      current = { path, header: line, lines: [line] };
    } else {
      current.lines.push(line);
    }
  }
  blocks.push(current);
  return blocks;
}

function render(blocks: Block[]): string {
  return blocks.flatMap((block) => block.lines).join("\n");
}

function formatValue(value: TomlValue): string {
  if (typeof value === "string") return JSON.stringify(value);
  return String(value);
}

function normalize(text: string): {
  text: string;
  restore: (out: string) => string;
} {
  const bom = text.startsWith("\uFEFF") ? "\uFEFF" : "";
  const body = bom ? text.slice(1) : text;
  const crlf = body.includes("\r\n");
  return {
    text: crlf ? body.replace(/\r\n/g, "\n") : body,
    restore: (out) => `${bom}${crlf ? out.replace(/\n/g, "\r\n") : out}`,
  };
}

function applyToBlock(
  blocks: Block[],
  sectionPath: string[] | null,
  entries: Record<string, TomlValue>,
): { found: boolean; changed: boolean } {
  const targetKey = sectionPath === null ? null : sectionPath.join(".");
  const block = blocks.find((candidate) => {
    if (sectionPath === null) return candidate.path === null;
    return candidate.path !== null && candidate.path.join(".") === targetKey;
  });
  if (!block) return { found: false, changed: false };

  let changed = false;
  const remaining: Record<string, TomlValue> = { ...entries };

  for (let i = 0; i < block.lines.length; i++) {
    const line = block.lines[i] as string;
    if (line.trim().startsWith("#")) continue;
    const match = KEY_RE.exec(line);
    if (!match) continue;
    const [, indent, rawKey, , comment] = match;
    const key = (rawKey as string).replace(/^"|"$/g, "");
    if (!Object.hasOwn(remaining, key)) continue;
    const next = `${indent}${rawKey} = ${formatValue(remaining[key] as TomlValue)}${comment ?? ""}`;
    if (next !== line) {
      block.lines[i] = next;
      changed = true;
    }
    delete remaining[key];
  }

  const missing = Object.keys(remaining);
  if (missing.length > 0) {

    let insertAt = block.lines.length;
    while (
      insertAt > 1 &&
      (block.lines[insertAt - 1] as string).trim() === ""
    ) {
      insertAt--;
    }
    const added = missing.map(
      (key) => `${key} = ${formatValue(remaining[key] as TomlValue)}`,
    );
    block.lines.splice(insertAt, 0, ...added);
    changed = true;
  }

  return { found: true, changed };
}

export function upsertTomlSection(
  text: string,
  sectionPath: string[],
  entries: Record<string, TomlValue>,
): TomlEditResult {
  const norm = normalize(text);
  assertParseable(norm.text);
  const blocks = splitBlocks(norm.text);

  const applied = applyToBlock(blocks, sectionPath, entries);
  if (applied.found) {
    return { text: norm.restore(render(blocks)), changed: applied.changed };
  }

  const header = `[${sectionPath.join(".")}]`;
  const body = Object.entries(entries).map(
    ([key, value]) => `${key} = ${formatValue(value)}`,
  );
  let out = norm.text;
  if (out.length > 0 && !out.endsWith("\n")) out += "\n";
  if (out.trim().length > 0) out += "\n";
  out += `${[header, ...body].join("\n")}\n`;
  return { text: norm.restore(out), changed: true };
}

export function upsertTomlTopLevel(
  text: string,
  entries: Record<string, TomlValue>,
): TomlEditResult {
  const norm = normalize(text);
  assertParseable(norm.text);
  const blocks = splitBlocks(norm.text);

  const applied = applyToBlock(blocks, null, entries);
  if (applied.found) {
    return { text: norm.restore(render(blocks)), changed: applied.changed };
  }

  const lines = Object.entries(entries).map(
    ([key, value]) => `${key} = ${formatValue(value)}`,
  );
  return {
    text: norm.restore(`${lines.join("\n")}\n${norm.text}`),
    changed: true,
  };
}

export function readTomlKey(
  text: string,
  sectionPath: string[] | null,
  key: string,
): string | null {
  const blocks = splitBlocks(normalize(text).text);
  const targetKey = sectionPath === null ? null : sectionPath.join(".");
  const block = blocks.find((candidate) => {
    if (sectionPath === null) return candidate.path === null;
    return candidate.path !== null && candidate.path.join(".") === targetKey;
  });
  if (!block) return null;
  for (const line of block.lines) {
    if (line.trim().startsWith("#")) continue;
    const match = KEY_RE.exec(line);
    if (!match) continue;
    const foundKey = (match[2] as string).replace(/^"|"$/g, "");
    if (foundKey === key) {
      const raw = (match[3] as string).trim();
      if (raw.startsWith('"') && raw.endsWith('"')) {
        try {
          return JSON.parse(raw) as string;
        } catch {
          return raw.slice(1, -1);
        }
      }
      return raw;
    }
  }
  return null;
}

export function removeTomlTopLevelKey(
  text: string,
  key: string,
): TomlEditResult {
  const norm = normalize(text);
  assertParseable(norm.text);
  const blocks = splitBlocks(norm.text);
  const block = blocks.find((candidate) => candidate.path === null);
  if (!block) return { text, changed: false };
  const kept = block.lines.filter((line) => {
    if (line.trim().startsWith("#")) return true;
    const match = KEY_RE.exec(line);
    if (!match) return true;
    return (match[2] as string).replace(/^"|"$/g, "") !== key;
  });
  if (kept.length === block.lines.length) return { text, changed: false };
  block.lines = kept;
  return { text: norm.restore(render(blocks)), changed: true };
}

export function removeTomlSection(
  text: string,
  sectionPath: string[],
): TomlEditResult {
  const norm = normalize(text);
  assertParseable(norm.text);
  const blocks = splitBlocks(norm.text);
  const targetKey = sectionPath.join(".");
  const kept = blocks.filter(
    (block) => !(block.path !== null && block.path.join(".") === targetKey),
  );
  if (kept.length === blocks.length) return { text, changed: false };
  return { text: norm.restore(render(kept)), changed: true };
}
