

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
      `  ${param}  (string, required) - resource reference: pass a path / name / id (resolved server-side)`,
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
    if (sub.description) line += ` - ${sub.description}`;
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

function responseFieldNames(spec: V1CommandSpec): string[] {
  const shape = (
    spec.response as unknown as { shape?: Record<string, unknown> }
  )?.shape;
  return shape ? Object.keys(shape) : [];
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
    lines.push(`  ${param}  (string, required) - path id`);
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
  lines.push(`idapt ${command} - ${summary}`);
  lines.push("");
  lines.push(`  ${spec.method} /api/v1${spec.path}`);
  if (spec.async) lines.push("  (long-running - supports --background)");

  const entry = catalogEntry(spec.command);
  const args = entry
    ? schemaArgLines(entry.inputSchema, spec.pathParams)
    : argLines(spec);
  if (args.length) {
    lines.push("");
    lines.push("Arguments:");
    lines.push(...args);
  }

  if (spec.argLocation === "multipart") {
    lines.push(
      "",
      "Local file (global flag):",
      "  --file <path>             the file to upload; also accepted as the",
      "                            first positional argument",
    );
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

  if (spec.responseKind === "list") {
    const fields = responseFieldNames(spec);
    lines.push("", "Shaping the list (global flags):");
    lines.push(
      "  --all                     walk every page, not just the first",
    );
    lines.push("  --columns <a,b>           pick and order columns");
    lines.push(
      "  --filter <field=value>    keep exact matches (`~` for contains)",
    );
    lines.push(
      "  --sort <field>            sort ascending (`-field` descending)",
    );
    if (fields.length > 0) {
      lines.push("", `Fields: ${fields.join(", ")}`);
    }
  }

  lines.push("");
  lines.push(`Playbook: idapt instructions ${resource}`);
  return lines.join("\n");
}

export function renderInstructions(resource: string): string {
  const playbook = getResourcePlaybook(resource);
  if (playbook) return playbook.instructions;
  return `No playbook authored for "${resource}". Known resources: ${listResources("cli").join(", ")}.`;
}

function examplePlaceholder(
  key: string,
  schema: JsonSchema | undefined,
): unknown {
  if (schema?.enum && schema.enum.length > 0) return schema.enum[0];
  const t = Array.isArray(schema?.type) ? schema?.type[0] : schema?.type;
  switch (t) {
    case "number":
    case "integer":
      return 0;
    case "boolean":
      return false;
    case "array":
      return [];
    case "object":
      return {};
    default:
      return `<${key}>`;
  }
}

export function requiredArgLabels(spec: V1CommandSpec): string[] {
  const labels = spec.pathParams.map((p) => `${p} (string)`);
  const entry = catalogEntry(spec.command);
  const props = entry?.inputSchema?.properties ?? {};
  const required = new Set(entry?.inputSchema?.required ?? []);
  const pp = new Set(spec.pathParams);
  for (const [key, sub] of Object.entries(props)) {
    if (pp.has(key) || !required.has(key)) continue;
    labels.push(`${key} (${schemaTypeLabel(sub)})`);
  }
  return labels;
}

export function buildExampleCall(
  spec: V1CommandSpec,
  options: { command?: string } = {},
): string {
  const action = options.command ?? spec.command;
  const entry = catalogEntry(spec.command);
  const authored = entry?.examples?.[0];
  if (authored) {
    return `{"action":"${authored.action}","args":${JSON.stringify(authored.args ?? {})}}`;
  }
  const args: Record<string, unknown> = {};
  for (const param of spec.pathParams) {
    args[param] = "<name or id>";
  }
  const props = entry?.inputSchema?.properties ?? {};
  const required = new Set(entry?.inputSchema?.required ?? []);
  const pp = new Set(spec.pathParams);
  for (const [key, sub] of Object.entries(props)) {
    if (pp.has(key) || !required.has(key)) continue;
    args[key] = examplePlaceholder(key, sub);
  }
  return `{"action":"${action}","args":${JSON.stringify(args)}}`;
}

export function renderCompactContract(
  spec: V1CommandSpec,
  options: { command?: string } = {},
): string {
  const required = requiredArgLabels(spec);
  const lines = [
    required.length > 0
      ? `Required: ${required.join(", ")}`
      : "Required: (no required arguments)",
    `Example: ${buildExampleCall(spec, options)}`,
  ];
  return lines.join("\n");
}

export function missingArgMessage(
  spec: V1CommandSpec,
  missing: string,
  options: { command?: string; isPathParam?: boolean } = {},
): string {
  const action = options.command ?? spec.command;
  const entry = catalogEntry(spec.command);
  const sub = entry?.inputSchema?.properties?.[missing];
  const isPath = options.isPathParam ?? spec.pathParams.includes(missing);
  const type = isPath
    ? "string, resource reference: pass a path / name / id"
    : `${schemaTypeLabel(sub ?? {})}, required`;
  return [
    `\`${action}\` is missing required argument: ${missing} (${type}).`,
    "",
    renderCompactContract(spec, options),
  ].join("\n");
}
