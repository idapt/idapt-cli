

import {
  resourceNoun,
  type V1CommandSpec,
} from "@idapt/api-contracts/v1/contracts";
import {
  colorEnabled,
  displayWidth,
  padTo,
  paint,
  relativeTime,
  sanitizeCell,
  terminalWidth,
  truncateTo,
} from "./text";

export type RenderMode = "json" | "jsonl" | "quiet" | "table" | "llm";

const LLM_MAX_ARRAY = 50;
const LLM_MAX_STRING = 4000;

const NOISY_FIELDS = new Set([
  "workspace_id",
  "billing_account",
  "owner",
  "resource_id",
  "search_vector",
]);

const TIME_FIELD = /_at$|^created$|^updated$/;

export interface RenderOptions {

  readonly color?: boolean;

  readonly columns?: readonly string[];

  readonly width?: number;

  readonly emptyLabel?: string;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

export function readPath(row: unknown, path: string): unknown {
  if (!path) return row;
  let cursor: unknown = row;
  for (const segment of path.split(".")) {
    if (!isRecord(cursor)) return undefined;
    cursor = cursor[segment];
  }
  return cursor;
}

function cell(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "boolean") return value ? "yes" : "no";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function humanCell(value: unknown, field: string): string {
  if (value === null || value === undefined) return "-";
  if (typeof value === "boolean") return value ? "yes" : "no";
  if (typeof value === "string" && value === "") return "-";
  if (TIME_FIELD.test(field) && value) return sanitizeCell(relativeTime(value));
  return sanitizeCell(cell(value));
}

const ATOMIC_FIELDS = new Set([
  "id",
  "resource_id",
  "slug",
  "qualified_slug",
  "key",
  "agent_id",
  "chat_id",
  "computer_id",
  "workspace_id",
]);

function isAtomic(field: string): boolean {
  const leaf = field.split(".").pop() ?? field;
  return ATOMIC_FIELDS.has(leaf);
}

function identityOf(row: Record<string, unknown>): string | undefined {
  for (const key of ["id", "resource_id", "slug", "key", "name"]) {
    const value = row[key];
    if (typeof value === "string" && value) return value;
    if (typeof value === "number") return String(value);
  }
  return undefined;
}

type Column = { header: string; field: string };

function resolveColumns(
  data: unknown,
  spec: V1CommandSpec,
  options: RenderOptions,
): Column[] {
  if (options.columns?.length) {
    return options.columns.map((field) => ({
      header: field.split(".").pop()?.toUpperCase() ?? field.toUpperCase(),
      field,
    }));
  }
  if (spec.columns?.length) return spec.columns.map((c) => ({ ...c }));

  const sample = Array.isArray(data) ? data[0] : data;
  if (!isRecord(sample)) return [{ header: "VALUE", field: "" }];

  const keys = Object.keys(sample).filter((k) => !NOISY_FIELDS.has(k));

  const preferred = ["id", "name", "title", "slug", "status", "state"];
  const ordered = [
    ...preferred.filter((k) => keys.includes(k)),
    ...keys.filter((k) => !preferred.includes(k)),
  ];
  return ordered
    .slice(0, 6)
    .map((k) => ({ header: k.toUpperCase(), field: k }));
}

function fitWidths(
  headers: readonly string[],
  body: readonly string[][],
  maxWidth: number,
  atomic: readonly boolean[] = [],
): number[] {
  const natural = headers.map((h, i) =>
    Math.max(displayWidth(h), ...body.map((r) => displayWidth(r[i] ?? ""))),
  );
  const gutter = 2 * (headers.length - 1);
  let total = natural.reduce((a, b) => a + b, 0) + gutter;
  if (total <= maxWidth) return natural;

  const widths = [...natural];

  const shrinkable = () =>
    widths
      .map((w, i) => ({ w, i }))
      .filter(({ i }) => !atomic[i] && (widths[i] as number) > 8);

  while (total > maxWidth) {
    const candidates = shrinkable();
    if (candidates.length === 0) break;
    const widest = candidates.reduce((a, b) => (b.w > a.w ? b : a));
    widths[widest.i] = (widths[widest.i] as number) - 1;
    total -= 1;
  }
  return widths;
}

function unwrapEnvelope(data: unknown): unknown {
  if (!isRecord(data)) return data;
  const arrays = Object.entries(data).filter(([, v]) => Array.isArray(v));
  if (arrays.length !== 1) return data;
  const [key, rows] = arrays[0] as [string, unknown[]];

  const ENVELOPE_KEYS = new Set([
    "items",
    "results",
    "data",
    "rows",
    "matches",
  ]);
  return ENVELOPE_KEYS.has(key) ? rows : data;
}

function unwrapKeyedMap(data: unknown): unknown {
  if (!isRecord(data)) return data;
  const values = Object.values(data);
  if (values.length === 0 || !values.every(isRecord)) return data;
  return Object.entries(data).map(([key, row]) => ({
    key,
    ...(row as Record<string, unknown>),
  }));
}

function renderTable(
  data: unknown,
  spec: V1CommandSpec,
  options: RenderOptions,
): string {
  const unwrapped = unwrapKeyedMap(unwrapEnvelope(data));
  const rows = Array.isArray(unwrapped)
    ? unwrapped
    : unwrapped === undefined
      ? []
      : [unwrapped];
  const color = options.color ?? false;

  if (rows.length === 0) {

    const label = options.emptyLabel ?? resourceNoun(spec.resource);
    return paint(`No ${label} found.`, "dim", color);
  }

  const cols = resolveColumns(unwrapped, spec, options);
  const headers = cols.map((c) => c.header.toUpperCase());
  const body = rows.map((row) =>
    cols.map((c) => humanCell(readPath(row, c.field), c.field)),
  );

  const widths = fitWidths(
    headers,
    body,
    options.width ?? terminalWidth(),
    cols.map((c) => isAtomic(c.field)),
  );
  const line = (parts: readonly string[], dim: boolean) =>
    parts
      .map((p, i) =>
        padTo(truncateTo(p, widths[i] as number), widths[i] as number),
      )
      .join("  ")
      .trimEnd()
      .replace(/^(.*)$/, (s) => (dim ? paint(s, "dim", color) : s));

  return [line(headers, true), ...body.map((r) => line(r, false))].join("\n");
}

function truncateForLlm(data: unknown): unknown {
  if (typeof data === "string") {
    return data.length > LLM_MAX_STRING
      ? `${data.slice(0, LLM_MAX_STRING)}…[${data.length - LLM_MAX_STRING} more chars]`
      : data;
  }
  if (Array.isArray(data)) {
    const head = data.slice(0, LLM_MAX_ARRAY).map(truncateForLlm);
    return data.length > LLM_MAX_ARRAY
      ? [...head, `…[${data.length - LLM_MAX_ARRAY} more items]`]
      : head;
  }
  if (isRecord(data)) {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(data)) out[k] = truncateForLlm(v);
    return out;
  }
  return data;
}

export function render(
  data: unknown,
  spec: V1CommandSpec,
  mode: RenderMode,
  options: RenderOptions = {},
): string {
  if (data instanceof Blob) return `<binary ${data.size} bytes>`;

  switch (mode) {
    case "json":
      return JSON.stringify(data, null, 2);
    case "jsonl":
      return Array.isArray(data)
        ? data.map((r) => JSON.stringify(r)).join("\n")
        : JSON.stringify(data);
    case "quiet": {

      const rows = Array.isArray(unwrapEnvelope(data))
        ? (unwrapEnvelope(data) as unknown[])
        : [data];
      return rows
        .map((r) => (isRecord(r) ? (identityOf(r) ?? "") : cell(r)))
        .filter(Boolean)
        .join("\n");
    }
    case "table":

      if (typeof data === "string") return data;
      return renderTable(data, spec, options);
    case "llm":

      return JSON.stringify(truncateForLlm(data));
    default:
      return JSON.stringify(data, null, 2);
  }
}

export function autoMode(isTty: boolean, requested?: RenderMode): RenderMode {
  if (requested) return requested;
  return isTty ? "table" : "json";
}

export { colorEnabled };

export function applyFilters(
  rows: readonly unknown[],
  filters: readonly string[],
): unknown[] {
  return rows.filter((row) =>
    filters.every((filter) => {
      const exact = filter.includes("=") && !filter.includes("~");
      const [field, expected] = filter.split(exact ? "=" : "~", 2);
      const actual = cell(readPath(row, (field ?? "").trim()));
      const needle = (expected ?? "").trim();
      return exact
        ? actual === needle
        : actual.toLowerCase().includes(needle.toLowerCase());
    }),
  );
}

export function filterField(filter: string): string {
  const exact = filter.includes("=") && !filter.includes("~");
  return (filter.split(exact ? "=" : "~", 2)[0] ?? "").trim();
}

export function sortField(sort: string): string {
  return sort.startsWith("-") ? sort.slice(1) : sort;
}

export function applySort(rows: readonly unknown[], sort: string): unknown[] {
  const descending = sort.startsWith("-");
  const field = descending ? sort.slice(1) : sort;
  return [...rows].sort((a, b) => {
    const av = readPath(a, field);
    const bv = readPath(b, field);
    const an = Number(av);
    const bn = Number(bv);
    const comparison =
      Number.isFinite(an) && Number.isFinite(bn)
        ? an - bn
        : cell(av).localeCompare(cell(bv));
    return descending ? -comparison : comparison;
  });
}
