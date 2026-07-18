

import { commandCatalog } from "@idapt/api-contracts/v1/command-catalog.generated";
import type { V1CommandSpec } from "@idapt/api-contracts/v1/contracts";
import type { ZodTypeAny } from "zod";
import { getResourcePlaybook, listResources } from "./catalog";

type JsonSchema = {
  type?: string | string[];
  properties?: Record<string, JsonSchema>;
  required?: string[];
  enum?: unknown[];
  default?: unknown;
  description?: string;
  items?: JsonSchema;
};

type CatalogEntry = {
  command: string;
  inputSchema: JsonSchema;
  outputSchema: JsonSchema | null;
  examples?: {
    title?: string;
    action: string;
    args?: Record<string, unknown>;
  }[];
};

function catalogEntry(command: string): CatalogEntry | undefined {
  return (commandCatalog as unknown as readonly CatalogEntry[]).find(
    (c) => c.command === command,
  );
}

function schemaTypeLabel(s: JsonSchema): string {
  if (s.enum && s.enum.length > 0) {
    return s.enum.map((v) => JSON.stringify(v)).join(" | ");
  }
  const t = Array.isArray(s.type) ? s.type.join("|") : s.type;
  if (t === "array") return `${s.items ? schemaTypeLabel(s.items) : "value"}[]`;
  return t ?? "value";
}

function schemaArgLines(
  inputSchema: JsonSchema | undefined,
  pathParams: readonly string[],
): string[] {
  const lines: string[] = [];
  for (const param of pathParams) {
    lines.push(
      `  ${param}  (string, required) — resource reference: pass a path / name / id (resolved server-side)`,
    );
  }
  const props = inputSchema?.properties ?? {};
  const required = new Set(inputSchema?.required ?? []);
  const pp = new Set(pathParams);
  for (const [key, sub] of Object.entries(props)) {
    if (pp.has(key)) continue;
    const parts = [
      schemaTypeLabel(sub),
      required.has(key) ? "required" : "optional",
    ];
    if (sub.default !== undefined) {
      parts.push(`default ${JSON.stringify(sub.default)}`);
    }
    let line = `  ${key}  (${parts.join(", ")})`;
    if (sub.description) line += ` — ${sub.description}`;
    lines.push(line);
  }
  return lines;
}

function returnLine(
  spec: V1CommandSpec,
  outputSchema: JsonSchema | null,
): string | null {
  if (spec.responseKind === "binary") return "Returns: raw bytes (binary)";
  if (!outputSchema) return null;
  const keys = Object.keys(outputSchema.properties ?? {});
  const shape = keys.length > 0 ? `{ ${keys.join(", ")} }` : "object";
  return `Returns: ${spec.responseKind === "list" ? `${shape}[]` : shape}`;
}

function typeHint(schema: ZodTypeAny): { name: string; optional: boolean } {
  let optional = false;
  let node: ZodTypeAny = schema;

  for (let i = 0; i < 4; i++) {
    const ctor = node?.constructor?.name ?? "";
    if (
      ctor === "ZodOptional" ||
      ctor === "ZodDefault" ||
      ctor === "ZodNullable"
    ) {
      optional = optional || ctor === "ZodOptional" || ctor === "ZodDefault";
      const inner = (
        node as unknown as { unwrap?: () => ZodTypeAny }
      ).unwrap?.();
      if (!inner) break;
      node = inner;
      continue;
    }
    break;
  }
  const map: Record<string, string> = {
    ZodString: "string",
    ZodNumber: "number",
    ZodBoolean: "boolean",
    ZodObject: "object",
    ZodArray: "array",
    ZodRecord: "object",
    ZodEnum: "enum",
  };
  return { name: map[node?.constructor?.name ?? ""] ?? "value", optional };
}

function argLines(spec: V1CommandSpec): string[] {
  const shape = (
    spec.request as unknown as { shape?: Record<string, ZodTypeAny> }
  ).shape;
  if (!shape) return [];
  const params = new Set(spec.pathParams);
  const lines: string[] = [];
  for (const param of spec.pathParams) {
    lines.push(`  ${param}  (string, required) — path id`);
  }
  for (const [key, field] of Object.entries(shape)) {
    if (params.has(key)) continue;
    const { name, optional } = typeHint(field);
    lines.push(`  ${key}  (${name}, ${optional ? "optional" : "required"})`);
  }
  return lines;
}

export type RenderHelpOptions = {

  command?: string;

  summary?: string;

  resource?: string;
};

export function renderHelp(
  spec: V1CommandSpec,
  options: RenderHelpOptions = {},
): string {
  const lines: string[] = [];
  const command = options.command ?? spec.command;
  const summary = options.summary ?? spec.summary;
  const resource = options.resource ?? spec.resource;
  lines.push(`idapt ${command} — ${summary}`);
  lines.push("");
  lines.push(`  ${spec.method} /api/v1${spec.path}`);
  if (spec.async) lines.push("  (long-running — supports --background)");

  const entry = catalogEntry(spec.command);
  const args = entry
    ? schemaArgLines(entry.inputSchema, spec.pathParams)
    : argLines(spec);
  if (args.length) {
    lines.push("");
    lines.push("Arguments:");
    lines.push(...args);
  }

  if (entry) {
    const ret = returnLine(spec, entry.outputSchema);
    if (ret) {
      lines.push("");
      lines.push(ret);
    }
    if (entry.examples && entry.examples.length > 0) {
      lines.push("");
      lines.push("Examples:");
      for (const ex of entry.examples) {
        lines.push(
          `  {action:"${ex.action}", args:${JSON.stringify(ex.args ?? {})}}${
            ex.title ? `  # ${ex.title}` : ""
          }`,
        );
      }
    }
  }

  if (spec.help) {
    lines.push("");
    lines.push(spec.help);
  }
  lines.push("");
  lines.push(`Playbook: idapt instructions ${resource}`);
  return lines.join("\n");
}

export function renderInstructions(resource: string): string {
  const playbook = getResourcePlaybook(resource);
  if (playbook) return playbook.instructions;
  return `No playbook authored for "${resource}". Known resources: ${listResources().join(", ")}.`;
}
