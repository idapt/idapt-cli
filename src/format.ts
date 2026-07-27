

import type { V1CommandSpec } from "@idapt/api-contracts/v1/contracts";

export type RenderMode = "json" | "jsonl" | "quiet" | "table" | "llm";

const LLM_MAX_ARRAY = 50;
const LLM_MAX_STRING = 4000;

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function cell(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function renderTable(data: unknown, spec: V1CommandSpec): string {
  const cols = spec.columns ??
    inferColumns(data) ?? [{ header: "VALUE", field: "" }];
  const rows = Array.isArray(data) ? data : [data];
  const header = cols.map((c) => c.header);
  const body = rows.map((row) =>
    cols.map((c) =>
      cell(c.field ? (row as Record<string, unknown>)?.[c.field] : row),
    ),
  );
  const widths = header.map((h, i) =>
    Math.max(h.length, ...body.map((r) => r[i].length), 0),
  );
  const fmt = (parts: string[]) =>
    parts
      .map((p, i) => p.padEnd(widths[i]))
      .join("  ")
      .trimEnd();
  return [fmt(header), ...body.map(fmt)].join("\n");
}

function inferColumns(
  data: unknown,
): { header: string; field: string }[] | undefined {
  const sample = Array.isArray(data) ? data[0] : data;
  if (!isRecord(sample)) return undefined;
  return Object.keys(sample).map((k) => ({
    header: k.toUpperCase(),
    field: k,
  }));
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
      const rows = Array.isArray(data) ? data : [data];
      return rows
        .map((r) => (isRecord(r) && "id" in r ? String(r.id) : cell(r)))
        .filter(Boolean)
        .join("\n");
    }
    case "table":
      return renderTable(data, spec);
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
