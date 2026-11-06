

const WIDE_RANGES: readonly [number, number][] = [
  [0x1100, 0x115f],
  [0x2e80, 0x303e],
  [0x3041, 0x33ff],
  [0x3400, 0x4dbf],
  [0x4e00, 0x9fff],
  [0xa000, 0xa4cf],
  [0xac00, 0xd7a3],
  [0xf900, 0xfaff],
  [0xfe10, 0xfe19],
  [0xfe30, 0xfe6f],
  [0xff00, 0xff60],
  [0xffe0, 0xffe6],
  [0x1f300, 0x1f64f],
  [0x1f900, 0x1f9ff],
  [0x1fa70, 0x1faff],
];

function isZeroWidth(cp: number): boolean {
  return (
    (cp >= 0x0300 && cp <= 0x036f) ||
    (cp >= 0xfe00 && cp <= 0xfe0f) ||
    cp === 0x200d ||
    cp === 0x200b
  );
}

function isWide(cp: number): boolean {
  return WIDE_RANGES.some(([lo, hi]) => cp >= lo && cp <= hi);
}

// biome-ignore lint/suspicious/noControlCharactersInRegex: matching ANSI escapes is the point
const ANSI_RE = /\u001b\[[0-9;]*m/g;

export function stripAnsi(text: string): string {
  return text.replace(ANSI_RE, "");
}

// biome-ignore lint/suspicious/noControlCharactersInRegex: removing control characters is the point
const CONTROL_RE = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g;

export function sanitizeCell(text: string): string {
  return text
    .replace(/[\t\n\r]/g, " ")
    .replace(CONTROL_RE, unicodeOk() ? "�" : "?");
}

export function displayWidth(text: string): number {
  let width = 0;
  for (const char of stripAnsi(text)) {
    const cp = char.codePointAt(0) ?? 0;
    if (isZeroWidth(cp)) continue;
    width += isWide(cp) ? 2 : 1;
  }
  return width;
}

export function padTo(text: string, width: number): string {
  const pad = width - displayWidth(text);
  return pad > 0 ? text + " ".repeat(pad) : text;
}

export function truncateTo(text: string, width: number): string {
  if (width <= 0) return "";
  if (displayWidth(text) <= width) return text;
  const marker = unicodeOk() ? "…" : "..";
  const budget = width - marker.length;
  let out = "";
  let used = 0;
  for (const char of stripAnsi(text)) {
    const cp = char.codePointAt(0) ?? 0;
    const w = isZeroWidth(cp) ? 0 : isWide(cp) ? 2 : 1;
    if (used + w > budget) break;
    out += char;
    used += w;
  }
  return out + marker;
}

export function column(
  rows: readonly (readonly [string, string])[],
  gutter = "  ",
): string[] {
  const width = Math.max(0, ...rows.map(([left]) => displayWidth(left)));
  return rows.map(([left, right]) =>
    right ? `${padTo(left, width)}${gutter}${right}` : left,
  );
}

export function unicodeOk(
  env: Record<string, string | undefined> = process.env,
): boolean {
  if (env.IDAPT_ASCII === "1") return false;
  if (process.platform !== "win32") {
    const locale = env.LC_ALL ?? env.LC_CTYPE ?? env.LANG ?? "";

    return locale === "" || /utf-?8/i.test(locale);
  }

  return Boolean(env.WT_SESSION || env.TERM_PROGRAM || env.ConEmuANSI);
}

export function symbols(
  env: Record<string, string | undefined> = process.env,
): {
  tick: string;
  cross: string;
  bullet: string;
  arrow: string;
  ellipsis: string;
} {
  return unicodeOk(env)
    ? { tick: "✓", cross: "✗", bullet: "•", arrow: "→", ellipsis: "…" }
    : { tick: "OK", cross: "x", bullet: "*", arrow: "->", ellipsis: "..." };
}

const CODES = {
  reset: "\u001b[0m",
  dim: "\u001b[2m",
  bold: "\u001b[1m",
  red: "\u001b[31m",
  green: "\u001b[32m",
  yellow: "\u001b[33m",
  cyan: "\u001b[36m",
} as const;

export type ColorName = "dim" | "bold" | "red" | "green" | "yellow" | "cyan";

export function colorEnabled(
  isTty: boolean,
  env: Record<string, string | undefined> = process.env,
  explicit?: boolean,
): boolean {
  if (explicit !== undefined) return explicit;
  if (env.NO_COLOR) return false;
  if (env.FORCE_COLOR && env.FORCE_COLOR !== "0") return true;
  return isTty;
}

export function paint(text: string, name: ColorName, enabled: boolean): string {
  return enabled ? `${CODES[name]}${text}${CODES.reset}` : text;
}

export function terminalWidth(fallback = 100): number {
  const columns = process.stdout?.columns;
  return typeof columns === "number" && columns >= 40 ? columns : fallback;
}

export function relativeTime(value: unknown, now = Date.now()): string {
  if (typeof value !== "string" && typeof value !== "number") return "";
  const then = typeof value === "number" ? value : Date.parse(value);
  if (!Number.isFinite(then)) return String(value);

  const seconds = Math.round((now - then) / 1000);
  if (seconds < 0) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.round(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.round(months / 12)}y ago`;
}
