

import { parseIdaptCommand } from "@idapt/api-contracts/idapt-command";

export interface ParsedInvocation {

  readonly pathTokens: readonly string[];

  readonly args: Record<string, unknown>;
  readonly background: boolean;
  readonly timeoutSeconds?: number;
  readonly help: boolean;
  readonly instructions: boolean;
}

export type ParseResult =
  | { ok: true; invocation: ParsedInvocation }
  | { ok: false; message: string };

const ENVELOPE_KEYS = new Set([
  "background",
  "timeout_seconds",
  "help",
  "instructions",
  "json",
]);

export function toSnakeKey(key: string): string {
  return key
    .replace(/-/g, "_")
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase();
}

export function parseInvocation(cmd: string): ParseResult {
  const parsed = parseIdaptCommand(cmd);
  if (!parsed.ok) return { ok: false, message: parsed.message };

  const { pathTokens, args, background, timeoutSeconds, help, instructions } =
    parsed.command;

  const out: Record<string, unknown> = {};

  if (typeof args.json === "string") {
    let obj: unknown;
    try {
      obj = JSON.parse(args.json);
    } catch {
      return { ok: false, message: "idapt: --json is not valid JSON" };
    }
    if (obj && typeof obj === "object" && !Array.isArray(obj)) {
      for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
        out[toSnakeKey(k)] = v;
      }
    } else {
      return { ok: false, message: "idapt: --json must be a JSON object" };
    }
  }

  for (const [k, v] of Object.entries(args)) {
    const sk = toSnakeKey(k);
    if (ENVELOPE_KEYS.has(sk)) continue;
    out[sk] = v;
  }

  return {
    ok: true,
    invocation: {

      pathTokens: [...pathTokens],
      args: out,
      background,
      timeoutSeconds,
      help,
      instructions,
    },
  };
}
