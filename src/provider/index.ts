

import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { dirname } from "node:path";
import { EXIT_ERROR, EXIT_OK, EXIT_VALIDATION } from "../exit-codes";
import { JsonEditError } from "./json-config";
import { TomlEditError } from "./toml";
import {
  DEFAULT_ENV_KEY,
  DEFAULT_PROVIDER_ID,
  findTool,
  openAIBaseUrl,
  type ProviderContext,
  TOOLS,
  type ToolDescriptor,
} from "./tools";

export interface ProviderIo {
  readonly print: (s: string) => void;
  readonly err: (s: string) => void;
  readonly env: Record<string, string | undefined>;

  readonly baseUrl: string;
  readonly asJson?: boolean;

  readonly readFile?: (path: string) => string | null;
  readonly writeFile?: (path: string, text: string) => void;
  readonly exists?: (path: string) => boolean;
  readonly backup?: (path: string) => void;
}

export const PROVIDER_USAGE = [
  "idapt provider <command> [tool]",
  "",
  "  setup <tool>   Configure a tool to use idapt as its model provider",
  "  list           Show supported tools and whether each is configured",
  "  verify <tool>  Check a tool's config is correct",
  "  revert <tool>  Restore the config saved before `setup`",
  "  remove <tool>  Remove idapt from a tool's config",
  "",
  `  Tools: ${TOOLS.map((t) => t.id).join(", ")}`,
  "  Flags: --model <id>   pin a default model in the tool's config",
].join("\n");

function contextFor(io: ProviderIo, model?: string): ProviderContext {
  return {
    baseUrl: openAIBaseUrl(io.baseUrl),
    providerId: DEFAULT_PROVIDER_ID,
    envKey: DEFAULT_ENV_KEY,
    ...(model ? { model } : {}),
  };
}

function readConfig(io: ProviderIo, path: string): string | null {
  if (io.readFile) return io.readFile(path);
  try {
    return readFileSync(path, "utf8");
  } catch {
    return null;
  }
}

function configExists(io: ProviderIo, path: string): boolean {
  if (io.exists) return io.exists(path);
  return existsSync(path);
}

function writeConfig(io: ProviderIo, path: string, text: string): void {
  if (io.writeFile) {
    io.writeFile(path, text);
    return;
  }
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, text, "utf8");
}

function backupConfig(io: ProviderIo, path: string): void {
  if (io.backup) {
    io.backup(path);
    return;
  }
  if (existsSync(path)) copyFileSync(path, `${path}.bak`);
}

function parseToolArgs(args: readonly string[]): {
  toolId: string | undefined;
  model: string | undefined;
} {
  let toolId: string | undefined;
  let model: string | undefined;
  for (let i = 0; i < args.length; i++) {
    const arg = args[i] as string;
    if (arg === "--model") {
      model = args[i + 1];
      i++;
      continue;
    }
    if (arg.startsWith("--model=")) {
      model = arg.slice("--model=".length);
      continue;
    }
    if (!arg.startsWith("-") && toolId === undefined) toolId = arg;
  }
  return { toolId, model };
}

function unknownTool(io: ProviderIo, toolId: string | undefined): number {
  io.err(
    `idapt provider: ${toolId ? `unknown tool "${toolId}"` : "missing <tool>"}. ` +
      `Supported: ${TOOLS.map((t) => t.id).join(", ")}.\n`,
  );
  return EXIT_VALIDATION;
}

function printNextSteps(
  io: ProviderIo,
  tool: ToolDescriptor,
  ctx: ProviderContext,
): void {
  io.print(
    `\nNext:\n${tool
      .nextSteps(ctx)
      .map((line) => `  ${line}`)
      .join("\n")}\n`,
  );
}

export function runProvider(args: readonly string[], io: ProviderIo): number {
  const [sub] = args;
  const rest = args.slice(1);

  if (!sub || sub === "help" || sub === "--help" || sub === "-h") {
    io.print(`${PROVIDER_USAGE}\n`);
    return sub ? EXIT_OK : EXIT_VALIDATION;
  }

  switch (sub) {
    case "list":
      return runList(io);
    case "setup":
      return runSetup(rest, io);
    case "verify":
      return runVerify(rest, io);
    case "revert":
      return runRevert(rest, io);
    case "remove":
      return runRemove(rest, io);
    default:
      io.err(`idapt provider: unknown command "${sub}"\n\n${PROVIDER_USAGE}\n`);
      return EXIT_VALIDATION;
  }
}

function runList(io: ProviderIo): number {
  const ctx = contextFor(io);
  const rows = TOOLS.map((tool) => {
    const path = tool.configPath(io.env);
    const text = readConfig(io, path);
    const configured = text !== null && tool.verify(text, ctx).length === 0;
    return { tool, path, exists: text !== null, configured };
  });
  if (io.asJson) {
    io.print(
      `${JSON.stringify(
        rows.map((row) => ({
          tool: row.tool.id,
          label: row.tool.label,
          config: row.path,
          exists: row.exists,
          configured: row.configured,
        })),
        null,
        2,
      )}\n`,
    );
    return EXIT_OK;
  }
  io.print(
    `${rows
      .map((row) => {
        const status = row.configured
          ? "configured"
          : row.exists
            ? "present, not configured"
            : "not found";
        return `${row.tool.id.padEnd(10)} ${status.padEnd(24)} ${row.path}`;
      })
      .join("\n")}\n`,
  );
  return EXIT_OK;
}

function runSetup(rest: readonly string[], io: ProviderIo): number {
  const { toolId, model } = parseToolArgs(rest);
  const tool = toolId ? findTool(toolId) : undefined;
  if (!tool) return unknownTool(io, toolId);

  const ctx = contextFor(io, model);
  const path = tool.configPath(io.env);
  const existing = readConfig(io, path) ?? "";

  let result: { text: string; changed: boolean };
  try {
    result = tool.apply(existing, ctx);
  } catch (error) {
    if (error instanceof TomlEditError || error instanceof JsonEditError) {
      io.err(
        `idapt provider: refusing to edit ${path} — ${error.message}\n` +
          "  Fix or move the file, then re-run.\n",
      );
      return EXIT_ERROR;
    }
    throw error;
  }

  if (!result.changed) {
    io.print(`Already configured: ${tool.label}\n  config: ${path}\n`);
    printNextSteps(io, tool, ctx);
    return EXIT_OK;
  }

  if (existing.trim() !== "") backupConfig(io, path);
  writeConfig(io, path, result.text);
  io.print(
    `Configured ${tool.label} to use idapt as its model provider.\n  config: ${path}${
      existing.trim() !== "" ? `\n  backup: ${path}.bak` : ""
    }\n`,
  );
  printNextSteps(io, tool, ctx);
  return EXIT_OK;
}

function runVerify(rest: readonly string[], io: ProviderIo): number {
  const { toolId } = parseToolArgs(rest);
  const tool = toolId ? findTool(toolId) : undefined;
  if (!tool) return unknownTool(io, toolId);

  const ctx = contextFor(io);
  const path = tool.configPath(io.env);
  const text = readConfig(io, path);
  if (text === null) {
    io.err(
      `idapt provider: ${path} does not exist. Run \`idapt provider setup ${tool.id}\`.\n`,
    );
    return EXIT_ERROR;
  }
  const issues = tool.verify(text, ctx);
  if (issues.length === 0) {
    io.print(`${tool.label} is configured correctly (${path}).\n`);
    return EXIT_OK;
  }
  io.err(
    `${tool.label} config problems (${path}):\n${issues
      .map((issue) => `  - ${issue}`)
      .join("\n")}\n`,
  );
  return EXIT_ERROR;
}

function runRevert(rest: readonly string[], io: ProviderIo): number {
  const { toolId } = parseToolArgs(rest);
  const tool = toolId ? findTool(toolId) : undefined;
  if (!tool) return unknownTool(io, toolId);

  const path = tool.configPath(io.env);
  const backupPath = `${path}.bak`;
  if (!configExists(io, backupPath)) {
    io.err(`idapt provider: no backup at ${backupPath}.\n`);
    return EXIT_ERROR;
  }
  const backupText = readConfig(io, backupPath) ?? "";
  writeConfig(io, path, backupText);
  io.print(`Restored ${path} from ${backupPath}.\n`);
  return EXIT_OK;
}

function runRemove(rest: readonly string[], io: ProviderIo): number {
  const { toolId } = parseToolArgs(rest);
  const tool = toolId ? findTool(toolId) : undefined;
  if (!tool) return unknownTool(io, toolId);

  const ctx = contextFor(io);
  const path = tool.configPath(io.env);
  const existing = readConfig(io, path);
  if (existing === null) {
    io.print(`Nothing to remove — ${path} does not exist.\n`);
    return EXIT_OK;
  }
  let result: { text: string; changed: boolean };
  try {
    result = tool.remove(existing, ctx);
  } catch (error) {
    if (error instanceof TomlEditError || error instanceof JsonEditError) {
      io.err(`idapt provider: refusing to edit ${path} — ${error.message}\n`);
      return EXIT_ERROR;
    }
    throw error;
  }
  if (!result.changed) {
    io.print(`idapt is not present in ${path}.\n`);
    return EXIT_OK;
  }
  backupConfig(io, path);
  writeConfig(io, path, result.text);
  io.print(`Removed idapt from ${path}.\n`);
  return EXIT_OK;
}
