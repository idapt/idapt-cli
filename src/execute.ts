

import type { V1CommandSpec } from "@idapt/api-contracts/v1/contracts";
import { findCommand, resolveCommand } from "./catalog";
import { type RenderMode, render } from "./format";
import { renderHelp, renderInstructions } from "./help";
import { parseInvocation } from "./parser";
import { mapArgsToV1, reconcileToV1 } from "./reconcile";
import type { AgentToolsRequest, AgentToolsTransport } from "./transport";

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

const defaultSleep = (ms: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));

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
): { path: string; rest: Record<string, unknown> } | { error: string } {
  let path = spec.path;
  const rest = { ...args };
  const pos = [...positionals];
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
    path = path.replace(`:${param}`, encodeURIComponent(String(value)));
  }
  if (pos.length > 0) {
    return { error: `unexpected extra arguments: ${pos.join(" ")}` };
  }
  return { path: `/api/v1${path}`, rest };
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
    return err(spec.command, 0, `invalid arguments: ${msg}`);
  }
  const payload = validated.data as Record<string, unknown>;

  if (spec.argLocation === "multipart") {
    for (const [k, v] of Object.entries(bound.rest)) {
      if (v instanceof Blob && payload[k] === undefined) payload[k] = v;
    }
  }

  const req: AgentToolsRequest = {
    method: spec.method,
    path: bound.path,
    signal: opts.signal,
    expectBinary: spec.responseKind === "binary",
    ...(spec.argLocation === "query"
      ? { query: payload as AgentToolsRequest["query"] }
      : {}),
    ...(spec.argLocation === "body" ? { body: payload } : {}),
    ...(spec.argLocation === "multipart"
      ? { multipart: payload as AgentToolsRequest["multipart"] }
      : {}),
  };

  const res = await opts.transport.request(req);

  if (res.status >= 400) {
    const message =
      (res.json &&
        typeof res.json === "object" &&
        "error" in res.json &&
        (res.json as { error?: { message?: string } }).error?.message) ||
      res.text ||
      `request failed (${res.status})`;
    return err(spec.command, res.status, String(message));
  }

  let data: unknown;
  let pagination: ExecuteResult["pagination"];
  switch (spec.responseKind) {
    case "binary": {

      if (res.blob) {
        const text = await res.blob.text();

        const looksBinary =
          text.includes(String.fromCharCode(0)) ||
          text.includes(String.fromCharCode(0xfffd));
        data = looksBinary ? res.blob : text;
      } else {
        data = res.text ?? null;
      }
      break;
    }
    case "list": {
      const env = res.json as
        | { data?: unknown[]; pagination?: ExecuteResult["pagination"] }
        | undefined;
      data = env?.data ?? [];
      pagination = env?.pagination;
      break;
    }
    case "deleted":
      data = res.json ?? null;
      break;
    default: {
      const env = res.json as { data?: unknown } | undefined;
      data = env?.data ?? res.json ?? null;
    }
  }

  if (spec.async && isOperationHandle(data) && !TERMINAL.has(data.status)) {
    if (background) {
      return {
        ok: true,
        command: spec.command,
        status: res.status,
        data,
        rendered: render(data, spec, mode),
        operationId: data.id,
        pending: true,
      };
    }
    return awaitOperation(spec, data.id, opts, mode);
  }

  return {
    ok: true,
    command: spec.command,
    status: res.status,
    data,
    pagination,
    rendered: render(data, spec, mode),
  };
}

async function awaitOperation(
  spec: V1CommandSpec,
  operationId: string,
  opts: ExecuteOptions,
  mode: RenderMode,
): Promise<ExecuteResult> {
  const sleep = opts.sleep ?? defaultSleep;
  const interval = opts.pollIntervalMs ?? 1500;
  const maxAttempts = opts.maxPollAttempts ?? 120;

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    if (opts.signal?.aborted) {
      return err(spec.command, 0, "operation wait aborted");
    }
    await sleep(interval);
    const res = await opts.transport.request({
      method: "GET",
      path: `/api/v1/operations/${encodeURIComponent(operationId)}`,
      signal: opts.signal,
    });
    if (res.status >= 400) {
      return err(
        spec.command,
        res.status,
        `failed to poll operation ${operationId} (${res.status})`,
      );
    }
    const op = (res.json as { data?: unknown } | undefined)?.data;
    if (!isOperationHandle(op) || !TERMINAL.has(op.status)) continue;

    if (op.status === "completed") {
      const result = (op as { result?: unknown }).result ?? null;
      return {
        ok: true,
        command: spec.command,
        status: 200,
        data: result,
        rendered: render(result, spec, mode),
        operationId,
      };
    }

    const message =
      (op as { error?: { message?: string } }).error?.message ??
      `operation ${op.status}`;
    return { ...err(spec.command, 0, message), operationId };
  }
  return err(
    spec.command,
    0,
    `operation ${operationId} did not finish after ${maxAttempts} polls (use --background and \`operation get\`)`,
  );
}
