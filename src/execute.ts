

import type { V1CommandSpec } from "@idapt/api-contracts/v1/contracts";
import {
  COMMAND_BINDINGS,
  type CommandBinding,
  IdaptError,
  NetworkError,
  awaitOperation as sdkAwaitOperation,
  executeCommand as sdkExecuteCommand,
} from "@idapt/sdk";
import { findCommand, resolveCommand } from "./catalog";
import { type RenderMode, render } from "./format";
import { renderHelp, renderInstructions } from "./help";
import { parseInvocation } from "./parser";
import { mapArgsToV1, reconcileToV1 } from "./reconcile";
import type { AgentToolsTransport } from "./transport";

export interface ExecuteOptions {
  readonly transport: AgentToolsTransport;

  readonly mode?: RenderMode;
  readonly signal?: AbortSignal;

  readonly background?: boolean;

  readonly pollIntervalMs?: number;

  readonly maxPollAttempts?: number;

  readonly sleep?: (ms: number) => Promise<void>;
}

export interface ExecuteResult {
  readonly ok: boolean;
  readonly command: string;
  readonly status: number;

  readonly data: unknown;

  readonly pagination?: { has_more: boolean; next_cursor: string | null };

  readonly rendered: string;

  readonly error?: string;

  readonly operationId?: string;

  readonly pending?: boolean;
}

const TERMINAL = new Set(["completed", "failed", "cancelled"]);

function isOperationHandle(
  data: unknown,
): data is { id: string; status: string; result?: unknown; error?: unknown } {
  return (
    typeof data === "object" &&
    data !== null &&
    typeof (data as { id?: unknown }).id === "string" &&
    typeof (data as { status?: unknown }).status === "string"
  );
}

function err(command: string, status: number, message: string): ExecuteResult {
  return {
    ok: false,
    command,
    status,
    data: null,
    rendered: message,
    error: message,
  };
}

function bindPath(
  spec: V1CommandSpec,
  positionals: readonly string[],
  args: Record<string, unknown>,
):
  | { pathValues: Record<string, string>; rest: Record<string, unknown> }
  | { error: string } {
  const rest = { ...args };
  const pos = [...positionals];
  const pathValues: Record<string, string> = {};
  for (const param of spec.pathParams) {
    let value: unknown = rest[param];
    if (value !== undefined) {
      delete rest[param];
    } else {
      value = pos.shift();
    }
    if (value === undefined || value === null || value === "") {
      return { error: `missing required path argument: ${param}` };
    }
    pathValues[param] = String(value);
  }
  if (pos.length > 0) {
    return { error: `unexpected extra arguments: ${pos.join(" ")}` };
  }
  return { pathValues, rest };
}

export async function execute(
  cmd: string,
  opts: ExecuteOptions,
): Promise<ExecuteResult> {
  const mode = opts.mode ?? "json";
  const parsed = parseInvocation(cmd);
  if (!parsed.ok) return err(cmd, 0, parsed.message);
  const inv = parsed.invocation;

  let resolved = resolveCommand(inv.pathTokens);
  let mappedArgs = inv.args;
  if (!resolved) {
    for (const n of [3, 2]) {
      if (inv.pathTokens.length < n) continue;
      const agentPath = inv.pathTokens.slice(0, n).join(" ");
      const v1 = reconcileToV1(agentPath);
      if (v1 === agentPath) continue;
      const candidate = resolveCommand([
        ...v1.split(" "),
        ...inv.pathTokens.slice(n),
      ]);
      if (candidate) {
        resolved = candidate;
        mappedArgs = mapArgsToV1(
          agentPath,
          candidate.spec.pathParams,
          inv.args,
        );
        break;
      }
    }
  }
  if (!resolved) {
    return err(
      inv.pathTokens.join(" "),
      0,
      `unknown command: ${inv.pathTokens.join(" ")} (try \`idapt help\`)`,
    );
  }
  const { spec, rest: positionals } = resolved;

  if (inv.help) {
    return {
      ok: true,
      command: spec.command,
      status: 0,
      data: null,
      rendered: renderHelp(spec),
    };
  }
  if (inv.instructions) {
    return {
      ok: true,
      command: spec.command,
      status: 0,
      data: null,
      rendered: renderInstructions(spec.resource),
    };
  }

  return runSpec(
    spec,
    positionals,
    mappedArgs,
    opts,
    mode,
    opts.background ?? inv.background,
  );
}

export async function executeCommand(
  commandName: string,
  args: Record<string, unknown>,
  opts: ExecuteOptions,
): Promise<ExecuteResult> {
  const mode = opts.mode ?? "json";
  const spec = findCommand(commandName);
  if (!spec) {
    return err(commandName, 0, `unknown command: ${commandName}`);
  }
  return runSpec(spec, [], args, opts, mode, opts.background ?? false);
}

async function runSpec(
  spec: V1CommandSpec,
  positionals: readonly string[],
  args: Record<string, unknown>,
  opts: ExecuteOptions,
  mode: RenderMode,
  background: boolean,
): Promise<ExecuteResult> {
  const bound = bindPath(spec, positionals, args);
  if ("error" in bound) return err(spec.command, 0, bound.error);

  const validated = spec.request.safeParse(bound.rest);
  if (!validated.success) {
    const msg = validated.error.issues
      .map((i) => `${i.path.join(".") || "(root)"}: ${i.message}`)
      .join("; ");

    return err(
      spec.command,
      0,
      `invalid arguments for \`${spec.command}\`: ${msg}. Run \`idapt help ${spec.command}\` for the contract.`,
    );
  }
  const payload: Record<string, unknown> = {
    ...(validated.data as Record<string, unknown>),
  };

  if (spec.argLocation === "multipart") {
    for (const [k, v] of Object.entries(bound.rest)) {
      if (v instanceof Blob && payload[k] === undefined) payload[k] = v;
    }
  }

  const binding: CommandBinding | undefined = COMMAND_BINDINGS[spec.command];
  const ctx = opts.transport.ctx;
  if (!binding || !ctx) {
    return err(
      spec.command,
      0,
      `\`${spec.command}\` is not runnable through this transport`,
    );
  }

  const sdkArgs = { ...bound.pathValues, ...payload };

  try {

    const raw = await sdkExecuteCommand<unknown>(binding, sdkArgs, ctx, {
      wait: false,
      bufferBinary: true,
      ...(opts.signal ? { signal: opts.signal } : {}),
    });

    if (spec.async && isOperationHandle(raw) && !TERMINAL.has(raw.status)) {
      if (background) {
        return {
          ok: true,
          command: spec.command,
          status: 202,
          data: raw,
          rendered: render(raw, spec, mode),
          operationId: raw.id,
          pending: true,
        };
      }
      try {
        const result = await sdkAwaitOperation<unknown>(binding, raw.id, ctx, {
          ...(opts.signal ? { signal: opts.signal } : {}),
          ...(opts.sleep ? { sleep: opts.sleep } : {}),
          ...(opts.pollIntervalMs !== undefined
            ? { pollIntervalMs: opts.pollIntervalMs }
            : {}),
          ...(opts.maxPollAttempts !== undefined
            ? { maxPollAttempts: opts.maxPollAttempts }
            : {}),
        });
        return {
          ok: true,
          command: spec.command,
          status: 200,
          data: result,
          rendered: render(result, spec, mode),
          operationId: raw.id,
        };
      } catch (e) {

        if (e instanceof NetworkError) throw e;
        if (e instanceof IdaptError) {
          return {
            ...err(spec.command, e.status, e.message),
            operationId: raw.id,
          };
        }
        throw e;
      }
    }

    let data: unknown = raw;
    let pagination: ExecuteResult["pagination"];
    if (spec.responseKind === "binary" && raw instanceof Blob) {

      const text = await raw.text();
      const looksBinary =
        text.includes(String.fromCharCode(0)) ||
        text.includes(String.fromCharCode(0xfffd));
      data = looksBinary ? raw : text;
    } else if (spec.responseKind === "list") {
      const env = raw as
        | { data?: unknown[]; pagination?: ExecuteResult["pagination"] }
        | undefined;
      data = env?.data ?? [];
      pagination = env?.pagination;
    }

    return {
      ok: true,
      command: spec.command,
      status: spec.responseKind === "created" ? 201 : 200,
      data,
      ...(pagination ? { pagination } : {}),
      rendered: render(data, spec, mode),
    };
  } catch (e) {

    if (e instanceof IdaptError && e.status >= 400) {
      return err(spec.command, e.status, e.message);
    }
    throw e;
  }
}
