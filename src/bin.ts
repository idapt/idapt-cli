#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { quoteToken } from "@idapt/api-contracts/idapt-command";
import { workspaceScopeOf } from "@idapt/api-contracts/v1/contracts";
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
import { loadActiveCredentials, runContextCommand } from "./auth/contexts";
import { listResources, resolveCommandForCli } from "./catalog";
import {
  COMPLETION_SHELLS,
  type CompletionShell,
  completeWords,
  completionInstallHint,
  completionScript,
  detectShell,
} from "./completion";
import { runShell } from "./computer/shell";
import { runConfig } from "./config";
import { confirmDestructive } from "./confirm";
import { renderHelpDoc, renderInstructionsDoc } from "./doc";
import { formatError, renderError } from "./errors";
import {
  EXIT_AUTH,
  EXIT_ERROR,
  EXIT_OK,
  EXIT_VALIDATION,
  exitCodeForStatus,
} from "./exit-codes";
import {
  flagNamesIn,
  type GlobalFlags,
  parseGlobalFlags,
  validateVerbFlags,
} from "./flags";
import { idsFromRows, rememberIds } from "./id-cache";
import { configDir } from "./idaptpaths";
import { autoMode, createFetchTransport, execute } from "./index";
import { runOpen } from "./open";
import { formatBytes, withProgress } from "./progress";
import { maybeNotify, runUpgrade } from "./update";
import { openLocalFilePart } from "./upload";
import { USER_AGENT, VERSION } from "./version";
import { resolveWorkspaceRef, runWorkspaceContext } from "./workspace-context";

const DEFAULT_BASE_URL = "https://idapt.app";

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
    const tok = argv[i] as string;
    if (tok.startsWith("--json=")) {
      const resolved = resolveVal(tok.slice("--json=".length));
      out.push(resolved === null ? tok : `--json=${resolved}`);
      continue;
    }
    if (tok === "--json" && i + 1 < argv.length) {
      out.push(tok);
      const resolved = resolveVal(argv[i + 1] as string);
      out.push(resolved === null ? (argv[i + 1] as string) : resolved);
      i++;
      continue;
    }
    out.push(tok);
  }
  return out;
}

const SECRET_FLAGS = ["--value", "--api-key"];

export function resolveSecretArgSource(
  argv: readonly string[],
  io: { readFile: (path: string) => string; readStdin: () => string },
): string[] {
  const out: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const tok = argv[i] as string;
    const next = argv[i + 1];
    if (!SECRET_FLAGS.includes(tok) || next === undefined) {
      out.push(tok);
      continue;
    }
    out.push(tok);
    if (next === "-") out.push(io.readStdin().trim());
    else if (next.startsWith("@")) out.push(io.readFile(next.slice(1)).trim());
    else out.push(next);
    i++;
  }
  return out;
}

export function splitTrailingDocWord(tokens: readonly string[]): {
  path: string[];
  doc: "help" | "instructions" | null;
} {
  const last = tokens.at(-1);
  if (last === "help" || last === "instructions") {
    return { path: tokens.slice(0, -1), doc: last };
  }
  return { path: [...tokens], doc: null };
}

export interface RunIo {
  argv: readonly string[];
  env: Record<string, string | undefined>;
  stdout: (s: string) => void;
  stderr: (s: string) => void;
  isTty: boolean;

  prompt?: (question: string) => Promise<string>;
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
  const { globals, rest, errors } = parseGlobalFlags(io.argv);
  if (errors.length > 0) {
    io.stderr(`${errors.map((e) => `idapt: ${e}`).join("\n")}\n`);
    return EXIT_VALIDATION;
  }

  const baseUrl =
    globals.apiUrl ??
    io.env.IDAPT_API_URL ??
    contextApiUrl(globals.context, io.env) ??
    DEFAULT_BASE_URL;

  if (globals.version || rest[0] === "version") {
    io.stdout(`${versionBlock(baseUrl)}\n`);
    return EXIT_OK;
  }

  if (rest.length === 0) {
    io.stdout(`${renderHelpDoc([], { env: io.env })}\n`);
    return globals.help || io.argv.length === 0 ? EXIT_OK : EXIT_OK;
  }

  const cmd = rest[0] as string;

  if (cmd === "help") {
    io.stdout(`${renderHelpDoc(rest.slice(1), { env: io.env })}\n`);
    return EXIT_OK;
  }
  if (cmd === "instructions") {
    io.stdout(`${renderInstructionsDoc(rest.slice(1))}\n`);
    return EXIT_OK;
  }

  if (cmd === "__complete") {
    for (const candidate of completeWords(rest.slice(1))) {
      io.stdout(`${candidate}\n`);
    }
    return EXIT_OK;
  }

  if (cmd === "completion") {
    const shell = rest[1];
    if (shell === "install") {
      io.stdout(`${completionInstallHint(detectShell(io.env))}\n`);
      return EXIT_OK;
    }
    if (!shell || !COMPLETION_SHELLS.includes(shell as CompletionShell)) {
      io.stderr(
        `idapt completion <${COMPLETION_SHELLS.join("|")}>\n` +
          "  idapt completion install   # print the one-liner for your shell\n",
      );
      return EXIT_ERROR;
    }
    io.stdout(completionScript(shell as CompletionShell));
    return EXIT_OK;
  }

  if (cmd === "upgrade" || cmd === "update") {
    return runUpgrade(
      { stdout: io.stdout, stderr: io.stderr, env: io.env },
      { check: rest.includes("--check"), next: rest.includes("--next") },
    );
  }

  if (cmd === "uninstall") {
    io.stdout(`${uninstallBlock()}\n`);
    return EXIT_OK;
  }

  const ctx = authCtx(io, baseUrl, globals.apiKey);
  if (cmd === "login") {
    return runLogin(ctx, parseLoginOpts(rest.slice(1), globals));
  }
  if (cmd === "logout") return runLogout(ctx);
  if (cmd === "whoami") return runStatus(ctx, globals.context);
  if (cmd === "auth") {
    const sub = rest[1];
    if (sub === "login") {
      return runLogin(ctx, parseLoginOpts(rest.slice(2), globals));
    }
    if (sub === "logout") return runLogout(ctx);
    if (sub === "status") return runStatus(ctx, globals.context);
    if (
      sub === "list" ||
      sub === "switch" ||
      sub === "rename" ||
      sub === "remove"
    ) {
      return runContextCommand(sub, rest.slice(2), {
        print: io.stdout,
        err: io.stderr,
      });
    }
    io.stderr(
      "idapt auth: use `login`, `logout`, `status` (alias: `idapt whoami`),\n" +
        "  or manage accounts: `list`, `switch <name>`, `rename <from> <to>`, `remove <name>`.\n",
    );
    return EXIT_ERROR;
  }

  if (cmd === "config") {
    return runConfig(rest.slice(1), {
      print: io.stdout,
      err: io.stderr,
      env: io.env,
      baseUrl,
    });
  }

  if (cmd === "app") {
    const appCred = await resolveCredential({
      apiKeyFlag: globals.apiKey,
      env: io.env,
      baseUrl,
      contextFlag: globals.context,
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

  if (cmd === "open") {
    return runOpen(rest.slice(1), {
      print: io.stdout,
      err: io.stderr,
      baseUrl,
      isTty: io.isTty,
    });
  }

  if (cmd === "workspace" || cmd === "agent") {
    const handled = await runAmbientContextSubverb(cmd, rest, io, {
      baseUrl,
      globals,
    });
    if (handled !== null) return handled;
  }

  if (cmd === "computer" && rest[1] === "shell") {
    return runComputerShell(rest, io, { baseUrl, globals });
  }

  const { path: docPath, doc: trailingDoc } = splitTrailingDocWord(rest);
  const wantsHelp = globals.help || trailingDoc === "help";
  const wantsInstructions =
    globals.instructions || trailingDoc === "instructions";

  const resolved = resolveCommandForCli(
    docPath.filter((t) => !t.startsWith("-")),
  );

  if (wantsInstructions) {
    const resource = resolved?.spec.resource ?? docPath[0];
    io.stdout(`${renderInstructionsDoc(resource ? [resource] : [])}\n`);
    return EXIT_OK;
  }
  if (wantsHelp) {
    io.stdout(
      `${renderHelpDoc(
        resolved
          ? [resolved.spec.resource, resolved.spec.verb]
          : docPath.filter((t) => !t.startsWith("-")),
        { env: io.env },
      )}\n`,
    );
    return EXIT_OK;
  }

  if (!resolved) {
    const tokens = docPath.filter((t) => !t.startsWith("-"));

    if (
      tokens.length === 1 &&
      listResources("cli").includes(tokens[0] as string)
    ) {
      io.stdout(`${renderHelpDoc(tokens, { env: io.env })}\n`);
      return EXIT_OK;
    }

    io.stderr(
      `${renderHelpDoc(tokens, { env: io.env, unknownIsError: true })}\n`,
    );
    return EXIT_VALIDATION;
  }

  const { spec, rest: positionals } = resolved;

  const declared = Object.keys(
    (spec.request as { shape?: Record<string, unknown> })?.shape ?? {},
  );
  const flagErrors = validateVerbFlags(
    flagNamesIn(rest).filter((f) => !RESERVED_INPUT_FLAGS.has(f)),
    [...declared, ...spec.pathParams, "json"],
    spec.command,
  );
  if (flagErrors.length > 0) {
    io.stderr(`${flagErrors.map((e) => `idapt: ${e}`).join("\n")}\n`);
    return EXIT_VALIDATION;
  }

  const credential = await resolveCredential({
    apiKeyFlag: globals.apiKey,
    env: io.env,
    baseUrl,
    contextFlag: globals.context,
    userAgent: USER_AGENT,
  }).catch(() => null);
  if (!credential) {
    io.stderr(`${AUTH_HINT}\n`);
    return EXIT_AUTH;
  }

  const transport = createFetchTransport({
    baseUrl,
    token: credential.token,
    userAgent: USER_AGENT,
    ...(globals.timeoutSeconds !== undefined
      ? { timeoutMs: globals.timeoutSeconds * 1000 }
      : {}),
    ...(globals.verbose
      ? { onTrace: (line: string) => io.stderr(`${line}\n`) }
      : {}),
  });

  const scope = workspaceScopeOf(spec);

  if (globals.workspace && scope === "none") {
    io.stderr(
      `idapt: \`${spec.command}\` is not workspace-scoped, so --workspace does not apply.\n` +
        "  It reads a catalog that is the same in every workspace, or acts on your account.\n",
    );
    return EXIT_VALIDATION;
  }
  if (globals.workspace && scope === "derived") {
    io.stderr(
      `idapt: \`${spec.command}\` takes its workspace from the resource you name.\n` +
        "  Passing --workspace too could disagree with it, so it is rejected. To act in\n" +
        "  another workspace, name a resource that lives there.\n",
    );
    return EXIT_VALIDATION;
  }
  if (globals.workspace && scope === "target") {
    io.stderr(
      `idapt: \`${spec.command}\` moves data between workspaces.\n` +
        "  Name the DESTINATION with --to <workspace>; --workspace is the workspace you operate in.\n",
    );
    return EXIT_VALIDATION;
  }
  if (globals.to && scope !== "target") {
    io.stderr(
      `idapt: \`${spec.command}\` does not take a destination workspace. Did you mean --workspace?\n`,
    );
    return EXIT_VALIDATION;
  }
  if (globals.file && spec.argLocation !== "multipart") {
    io.stderr(
      `idapt: \`${spec.command}\` does not take a file. --file is for upload verbs\n` +
        "  (drive upload, blobs put, computer upload, inference transcribe).\n",
    );
    return EXIT_VALIDATION;
  }

  const creds = loadActiveCredentials({
    ...(globals.context ? { flag: globals.context } : {}),
    env: io.env,
  });
  const workspaceRef =
    globals.workspace ?? io.env.IDAPT_WORKSPACE ?? creds.defaultWorkspaceId;

  let scopedWorkspaceId: string | undefined;
  if (scope === "scope" && workspaceRef) {

    if (credential.isWorkspaceScoped) {
      if (globals.workspace) {
        io.stderr(
          "idapt: this credential is pinned to a single workspace, so --workspace is ignored.\n",
        );
      }
    } else {
      const lookup = await resolveWorkspaceRef(transport, workspaceRef);
      if (!lookup.ok) {
        io.stderr(`${lookup.error}\n`);
        return lookup.code;
      }
      scopedWorkspaceId = lookup.resourceId;
    }
  }
  let destinationWorkspaceId: string | undefined;
  if (scope === "target" && globals.to) {
    const lookup = await resolveWorkspaceRef(transport, globals.to);
    if (!lookup.ok) {
      io.stderr(`${lookup.error}\n`);
      return lookup.code;
    }
    destinationWorkspaceId = lookup.resourceId;
  }

  if (spec.destructive === "irreversible" && !globals.yes) {
    const decision = await confirmDestructive(spec, positionals, {
      isTty: io.isTty,
      err: io.stderr,
      ...(io.prompt ? { prompt: io.prompt } : {}),
    });
    if (!decision.confirmed) {
      io.stderr(`${decision.message}\n`);
      return decision.code;
    }
  }

  const fileIo = {
    readFile: (p: string) => readFileSync(p, "utf-8"),
    readStdin: () => readFileSync(0, "utf-8"),
  };
  const restWithJson = resolveSecretArgSource(
    resolveJsonArgSource(rest, fileIo),
    fileIo,
  );

  const agentRef = globals.agent ?? creds.defaultAgentId;
  let defaultAgentMemoryBoxId = globals.agent
    ? undefined
    : creds.defaultAgentMemoryBoxId;
  if (
    !defaultAgentMemoryBoxId &&
    !globals.agent &&
    spec.resource === "memory"
  ) {
    defaultAgentMemoryBoxId = await resolveDefaultMemoryBox(transport);
  }

  const mode = autoMode(io.isTty, globals.output);
  const progressIo = {
    spec,
    isTty: io.isTty,
    mode,
    err: io.stderr,
    quiet: globals.output === "quiet",
  };

  let filePart: File | undefined;
  let uploadLabel: string | undefined;
  if (globals.file) {
    try {
      const part = await openLocalFilePart(globals.file);
      filePart = part.file;
      uploadLabel = `uploading ${part.name} (${formatBytes(part.size)})`;
    } catch (error) {
      io.stderr(
        `idapt: ${error instanceof Error ? error.message : String(error)}\n`,
      );
      return EXIT_VALIDATION;
    }
  }

  const runExecute = (signal: AbortSignal) =>
    execute(buildCommandFromArgv(restWithJson), {
      transport,
      mode,
      signal,
      ...(filePart ? { filePart } : {}),
      ...(globals.background ? { background: true } : {}),
      ...(scopedWorkspaceId ? { defaultWorkspaceRef: scopedWorkspaceId } : {}),
      ...(destinationWorkspaceId
        ? { destinationWorkspaceRef: destinationWorkspaceId }
        : {}),
      ...(agentRef ? { defaultAgentRef: agentRef } : {}),
      ...(defaultAgentMemoryBoxId ? { defaultAgentMemoryBoxId } : {}),
      ...(globals.all ? { fetchAllPages: true } : {}),
      ...(globals.columns ? { columns: globals.columns } : {}),
      ...(globals.filter ? { filters: globals.filter } : {}),
      ...(globals.sort ? { sort: globals.sort } : {}),
      ...(globals.noColor || io.env.NO_COLOR ? { color: false } : {}),
    });

  const result = await withProgress(
    uploadLabel ? { ...progressIo, label: uploadLabel } : progressIo,
    runExecute,
  ).catch((error: unknown) => ({ thrown: error }) as const);

  if ("thrown" in result) {
    io.stderr(
      `${renderError(
        formatError(result.thrown, {
          credentialOrigin: credential.source,
          command: spec.command,
          baseUrl,
        }),
      )}\n`,
    );
    return EXIT_ERROR;
  }

  if (result.ok) {
    if (result.rendered) io.stdout(`${result.rendered}\n`);

    if (spec.responseKind === "list" && Array.isArray(result.data)) {
      rememberIds(spec.resource, idsFromRows(result.data));
    }

    if (result.pageHint && mode === "table") io.stderr(`${result.pageHint}\n`);
    if (result.operationId && result.pending) {
      io.stderr(
        `\nStill running in the background.\n  Check it with: idapt operation get ${result.operationId}\n`,
      );
    }
  } else {
    io.stderr(
      `${renderError(

        formatError(
          result.cause ?? new Error(result.error ?? result.rendered),
          {
            credentialOrigin: credential.source,
            command: spec.command,
            baseUrl,
          },
        ),
      )}\n`,
    );
  }

  await maybeNotify({ stderr: io.stderr, env: io.env, isTty: io.isTty });
  return result.ok ? EXIT_OK : exitCodeForStatus(result.status);
}

const RESERVED_INPUT_FLAGS = new Set(["json", "background", "timeout-seconds"]);

async function runAmbientContextSubverb(
  cmd: "workspace" | "agent",
  rest: readonly string[],
  io: RunIo,
  opts: { baseUrl: string; globals: GlobalFlags },
): Promise<number | null> {
  const sub = rest[1];
  if (sub !== "use" && sub !== "current" && sub !== "clear") return null;

  const runner = cmd === "workspace" ? runWorkspaceContext : runAgentContext;
  if (sub === "current" || sub === "clear") {
    return runner(sub, rest.slice(2), undefined, {
      print: io.stdout,
      err: io.stderr,
    });
  }
  const cred = await resolveCredential({
    apiKeyFlag: opts.globals.apiKey,
    env: io.env,
    baseUrl: opts.baseUrl,
    contextFlag: opts.globals.context,
    userAgent: USER_AGENT,
  }).catch(() => null);
  const transport = cred
    ? createFetchTransport({
        baseUrl: opts.baseUrl,
        token: cred.token,
        userAgent: USER_AGENT,
      })
    : undefined;
  return runner(sub, rest.slice(2), transport, {
    print: io.stdout,
    err: io.stderr,
  });
}

async function runComputerShell(
  rest: readonly string[],
  io: RunIo,
  opts: { baseUrl: string; globals: GlobalFlags },
): Promise<number> {
  const target = rest[2];
  if (!target) {
    io.stderr(
      "idapt computer shell: which computer?\n" +
        "  idapt computer shell <[user@]computer> [--tmux] [--run-as <user>]\n",
    );
    return EXIT_VALIDATION;
  }
  const cred = await resolveCredential({
    apiKeyFlag: opts.globals.apiKey,
    env: io.env,
    baseUrl: opts.baseUrl,
    contextFlag: opts.globals.context,
    userAgent: USER_AGENT,
  }).catch(() => null);
  if (!cred) {
    io.stderr(`${AUTH_HINT}\n`);
    return EXIT_AUTH;
  }
  const flags = rest.slice(3);
  const flagValue = (name: string): string | undefined => {
    const i = flags.indexOf(`--${name}`);
    return i >= 0 ? flags[i + 1] : undefined;
  };
  return runShell(
    createFetchTransport({
      baseUrl: opts.baseUrl,
      token: cred.token,
      userAgent: USER_AGENT,
    }),
    {
      target,
      tmux: flags.includes("--tmux"),
      ...(flagValue("run-as") ? { runAs: flagValue("run-as") } : {}),
      ...(opts.globals.workspace ? { workspace: opts.globals.workspace } : {}),
    },
    { stdin: process.stdin, stdout: process.stdout, err: io.stderr },
  );
}

function contextApiUrl(
  contextFlag: string | undefined,
  env: Record<string, string | undefined>,
): string | undefined {
  try {
    return loadActiveCredentials({
      ...(contextFlag ? { flag: contextFlag } : {}),
      env,
    }).apiUrl;
  } catch {
    return undefined;
  }
}

function versionBlock(baseUrl: string): string {
  return [
    `idapt ${VERSION}`,
    `node ${process.versions.node} on ${process.platform}-${process.arch}`,
    `api ${baseUrl}`,
    `config ${configDir()}`,
  ].join("\n");
}

function uninstallBlock(): string {
  return [
    "To remove the idapt CLI:",
    "",
    "  npm uninstall -g @idapt/cli",
    "",
    "That leaves your stored sign-in in place. To remove it too, first run:",
    "",
    "  idapt logout",
  ].join("\n");
}

function parseLoginOpts(
  args: readonly string[],
  globals: GlobalFlags,
): {
  device?: boolean;
  web?: boolean;
  apiKeyStdin?: boolean;
  context?: string;
} {
  return {
    device: args.includes("--device"),
    web: args.includes("--web"),
    apiKeyStdin: args.includes("--api-key-stdin"),
    ...(globals.context ? { context: globals.context } : {}),
  };
}

export function isCliEntrypoint(entryPath: string | undefined): boolean {
  if (!entryPath) return false;
  return (
    /bin(\.[cm]?[jt]s)?$/.test(entryPath) || /(?:^|[\\/])idapt$/.test(entryPath)
  );
}

/* c8 ignore start — process wiring, exercised via `run()` in tests. */

async function finish(code: number): Promise<void> {
  process.exitCode = code;
  const timer = setTimeout(() => process.exit(code), 2000);
  timer.unref();
  try {
    const undici = (globalThis as { [k: symbol]: unknown })[
      Symbol.for("undici.globalDispatcher.1")
    ] as { close?: () => Promise<void> } | undefined;
    await undici?.close?.();
  } catch {

  }
}

async function main(): Promise<void> {
  const code = await run({
    argv: process.argv.slice(2),
    env: process.env,
    stdout: (s) => process.stdout.write(s),
    stderr: (s) => process.stderr.write(s),
    isTty: Boolean(process.stdout.isTTY),
  }).catch((error: unknown) => {
    process.stderr.write(`${renderError(formatError(error))}\n`);
    return EXIT_ERROR;
  });
  await finish(code);
}

if (typeof process !== "undefined" && isCliEntrypoint(process.argv[1])) {
  void main();
}
/* c8 ignore stop */
