#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { quoteToken } from "@idapt/api-contracts/idapt-command";
import { resolveDefaultMemoryBox, runAgentContext } from "./agent-context";
import { runApp } from "./app";
import {
  AUTH_HINT,
  type AuthCtx,
  resolveCredential,
  runLogin,
  runLogout,
  runStatus,
} from "./auth";
import { loadCredentials } from "./auth/credentials";
import { renderHelpDoc, renderInstructionsDoc } from "./doc";
import {
  autoMode,
  createFetchTransport,
  execute,
  type RenderMode,
} from "./index";
import { maybeNotify, runUpgrade } from "./update";
import { USER_AGENT, VERSION } from "./version";
import { runWorkspaceContext } from "./workspace-context";

const DEFAULT_BASE_URL = "https://idapt.app";
const OUTPUT_MODES = new Set<RenderMode>(["json", "jsonl", "table", "quiet"]);

export function extractOutputMode(argv: readonly string[]): {
  mode?: RenderMode;
  rest: string[];
} {
  const rest: string[] = [];
  let mode: RenderMode | undefined;
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--output" || a === "-o") {
      const v = argv[++i];
      if (v && OUTPUT_MODES.has(v as RenderMode)) mode = v as RenderMode;
      continue;
    }
    const inline = a.startsWith("--output=")
      ? a.slice("--output=".length)
      : null;
    if (inline && OUTPUT_MODES.has(inline as RenderMode)) {
      mode = inline as RenderMode;
      continue;
    }
    rest.push(a);
  }
  return { mode, rest };
}

export function extractValueFlag(
  argv: readonly string[],
  name: string,
): { value?: string; rest: string[] } {
  const rest: string[] = [];
  let value: string | undefined;
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === name) {
      value = argv[++i];
      continue;
    }
    if (a.startsWith(`${name}=`)) {
      value = a.slice(name.length + 1);
      continue;
    }
    rest.push(a);
  }
  return { value, rest };
}

export function buildCommandFromArgv(argv: readonly string[]): string {
  return ["idapt", ...argv.map(quoteToken)].join(" ");
}

export function resolveJsonArgSource(
  argv: readonly string[],
  io: { readFile: (path: string) => string; readStdin: () => string },
): string[] {
  const resolveVal = (val: string): string | null => {
    if (val === "-") return io.readStdin();
    if (val.startsWith("@")) return io.readFile(val.slice(1));
    return null;
  };
  const out: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const tok = argv[i];
    if (tok.startsWith("--json=")) {
      const resolved = resolveVal(tok.slice("--json=".length));
      out.push(resolved === null ? tok : `--json=${resolved}`);
      continue;
    }
    if (tok === "--json" && i + 1 < argv.length) {
      out.push(tok);
      const resolved = resolveVal(argv[i + 1]);
      out.push(resolved === null ? argv[i + 1] : resolved);
      i++;
      continue;
    }
    out.push(tok);
  }
  return out;
}

export function isDocCommand(rest: readonly string[]): boolean {
  if (rest[0] === "help" || rest[0] === "instructions") return true;
  return rest.some(
    (a) => a === "--help" || a === "-h" || a === "--instructions",
  );
}

export interface RunIo {
  argv: readonly string[];
  env: Record<string, string | undefined>;
  stdout: (s: string) => void;
  stderr: (s: string) => void;
  isTty: boolean;
}

function authCtx(io: RunIo, baseUrl: string, apiKeyFlag?: string): AuthCtx {
  return {
    out: io.stderr,
    print: io.stdout,
    env: io.env,
    isTty: io.isTty,
    baseUrl,
    apiKeyFlag,
  };
}

export async function run(io: RunIo): Promise<number> {
  const { mode: requested, rest: r1 } = extractOutputMode(io.argv);
  const { value: apiKeyFlag, rest: r2 } = extractValueFlag(r1, "--api-key");
  const { value: apiUrlFlag, rest: r3 } = extractValueFlag(r2, "--api-url");

  const { value: agentFlag, rest } = extractValueFlag(r3, "--agent");
  const baseUrl = apiUrlFlag ?? io.env.IDAPT_API_URL ?? DEFAULT_BASE_URL;
  const cmd = rest[0];

  if (cmd === "version" || cmd === "--version") {
    io.stdout(`idapt ${VERSION}\n`);
    return 0;
  }

  if (rest.length === 0) {
    io.stderr("idapt: missing command. Try `idapt help`.\n");
    return 1;
  }

  if (cmd === "help") {
    io.stdout(`${renderHelpDoc(rest.slice(1))}\n`);
    return 0;
  }
  if (cmd === "instructions") {
    io.stdout(`${renderInstructionsDoc(rest.slice(1))}\n`);
    return 0;
  }

  if (cmd === "upgrade" || cmd === "update") {
    return runUpgrade(
      { stdout: io.stdout, stderr: io.stderr, env: io.env },
      { check: rest.includes("--check"), next: rest.includes("--next") },
    );
  }

  const ctx = authCtx(io, baseUrl, apiKeyFlag);
  if (cmd === "login") {
    return runLogin(ctx, parseLoginOpts(rest.slice(1)));
  }
  if (cmd === "logout") {
    return runLogout(ctx);
  }
  if (cmd === "whoami") {
    return runStatus(ctx);
  }
  if (cmd === "auth") {
    const sub = rest[1];
    if (sub === "login") return runLogin(ctx, parseLoginOpts(rest.slice(2)));
    if (sub === "logout") return runLogout(ctx);
    if (sub === "status") return runStatus(ctx);
    io.stderr(
      "idapt auth: use `login`, `logout`, or `status` (alias: `idapt whoami`).\n",
    );
    return 1;
  }

  if (cmd === "app") {
    const appCred = await resolveCredential({
      apiKeyFlag,
      env: io.env,
      baseUrl,
      userAgent: USER_AGENT,
    }).catch(() => null);
    return runApp(
      {
        out: io.stderr,
        print: io.stdout,
        baseUrl,
        token: appCred?.token,
      },
      rest.slice(1),
    );
  }

  if (cmd === "workspace") {
    const sub = rest[1];
    if (sub === "current" || sub === "clear") {

      return runWorkspaceContext(sub, rest.slice(2), undefined, {
        print: io.stdout,
        err: io.stderr,
      });
    }
    if (sub === "use") {
      const wsCred = await resolveCredential({
        apiKeyFlag,
        env: io.env,
        baseUrl,
        userAgent: USER_AGENT,
      }).catch(() => null);
      const wsTransport = wsCred
        ? createFetchTransport({
            baseUrl,
            token: wsCred.token,
            userAgent: USER_AGENT,
          })
        : undefined;
      return runWorkspaceContext(sub, rest.slice(2), wsTransport, {
        print: io.stdout,
        err: io.stderr,
      });
    }

  }

  if (cmd === "agent") {
    const sub = rest[1];
    if (sub === "current" || sub === "clear") {

      return runAgentContext(sub, rest.slice(2), undefined, {
        print: io.stdout,
        err: io.stderr,
      });
    }
    if (sub === "use") {
      const agCred = await resolveCredential({
        apiKeyFlag,
        env: io.env,
        baseUrl,
        userAgent: USER_AGENT,
      }).catch(() => null);
      const agTransport = agCred
        ? createFetchTransport({
            baseUrl,
            token: agCred.token,
            userAgent: USER_AGENT,
          })
        : undefined;
      return runAgentContext(sub, rest.slice(2), agTransport, {
        print: io.stdout,
        err: io.stderr,
      });
    }

  }

  const doc = isDocCommand(rest);
  let token: string | undefined;
  const resolved = await resolveCredential({
    apiKeyFlag,
    env: io.env,
    baseUrl,
    userAgent: USER_AGENT,
  }).catch(() => null);
  if (resolved) {
    token = resolved.token;
  } else if (doc) {
    token = "";
  } else {
    io.stderr(`${AUTH_HINT}\n`);
    return 1;
  }

  const transport = createFetchTransport({
    baseUrl,
    token,
    userAgent: USER_AGENT,
  });

  const restWithJson = resolveJsonArgSource(rest, {
    readFile: (p) => readFileSync(p, "utf-8"),
    readStdin: () => readFileSync(0, "utf-8"),
  });

  const creds = loadCredentials();
  const ambientWorkspace = creds.defaultWorkspaceId;

  const defaultAgentRef = agentFlag ?? creds.defaultAgentId;
  let defaultAgentMemoryBoxId = agentFlag
    ? undefined
    : creds.defaultAgentMemoryBoxId;

  if (!defaultAgentMemoryBoxId && !agentFlag && cmd === "memory" && !doc) {
    defaultAgentMemoryBoxId = await resolveDefaultMemoryBox(transport);
  }
  const result = await execute(buildCommandFromArgv(restWithJson), {
    transport,
    mode: autoMode(io.isTty, requested),
    ...(ambientWorkspace ? { defaultWorkspaceRef: ambientWorkspace } : {}),
    ...(defaultAgentRef ? { defaultAgentRef } : {}),
    ...(defaultAgentMemoryBoxId ? { defaultAgentMemoryBoxId } : {}),
  });

  if (result.ok) {
    io.stdout(`${result.rendered}\n`);
  } else {
    let msg = result.error ?? result.rendered;

    if (
      cmd === "memory" &&
      !defaultAgentMemoryBoxId &&
      /path argument/.test(msg)
    ) {
      msg +=
        "\nNo agent selected. Run `idapt agent use <name>` to target its Memory, or pass a box id.";
    }
    io.stderr(`${msg}\n`);
  }

  await maybeNotify({ stderr: io.stderr, env: io.env, isTty: io.isTty });
  return result.ok ? 0 : 1;
}

function parseLoginOpts(args: readonly string[]): {
  device?: boolean;
  web?: boolean;
  apiKeyStdin?: boolean;
} {
  return {
    device: args.includes("--device"),
    web: args.includes("--web"),
    apiKeyStdin: args.includes("--api-key-stdin"),
  };
}

export function isCliEntrypoint(entryPath: string | undefined): boolean {
  if (!entryPath) return false;
  return (
    /bin(\.[cm]?[jt]s)?$/.test(entryPath) || /(?:^|[\\/])idapt$/.test(entryPath)
  );
}

/* c8 ignore start — process wiring, exercised via `run()` in tests. */
async function main(): Promise<void> {
  const code = await run({
    argv: process.argv.slice(2),
    env: process.env,
    stdout: (s) => process.stdout.write(s),
    stderr: (s) => process.stderr.write(s),
    isTty: Boolean(process.stdout.isTTY),
  });
  process.exit(code);
}

if (typeof process !== "undefined" && isCliEntrypoint(process.argv[1])) {
  void main();
}
/* c8 ignore stop */
