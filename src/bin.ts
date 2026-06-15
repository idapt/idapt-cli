#!/usr/bin/env node

import { quoteToken } from "@idapt/api-contracts/idapt-command";
import {
  AUTH_HINT,
  type AuthCtx,
  resolveCredential,
  runLogin,
  runLogout,
  runStatus,
} from "./auth";
import { renderHelpDoc, renderInstructionsDoc } from "./doc";
import {
  autoMode,
  createFetchTransport,
  execute,
  type RenderMode,
} from "./index";
import { maybeNotify, runUpgrade } from "./update";
import { USER_AGENT, VERSION } from "./version";

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
  const { value: apiUrlFlag, rest } = extractValueFlag(r2, "--api-url");
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
  const result = await execute(buildCommandFromArgv(rest), {
    transport,
    mode: autoMode(io.isTty, requested),
  });

  if (result.ok) io.stdout(`${result.rendered}\n`);
  else io.stderr(`${result.error ?? result.rendered}\n`);

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
