

import type { V1CommandSpec } from "@idapt/api-contracts/v1/contracts";
import type { ZodTypeAny } from "zod";
import { getResourcePlaybook, listResources } from "./catalog";

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
  const args = argLines(spec);
  if (args.length) {
    lines.push("");
    lines.push("Arguments:");
    lines.push(...args);
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
