

import { loadContexts, saveContexts } from "./auth/contexts";
import { EXIT_OK, EXIT_VALIDATION } from "./exit-codes";
import { configDir } from "./idaptpaths";

export interface ConfigIo {
  readonly print: (s: string) => void;
  readonly err: (s: string) => void;
  readonly env: Record<string, string | undefined>;
  readonly baseUrl: string;
}

const SETTABLE = new Set(["api-url", "workspace", "agent"]);

export const CONFIG_USAGE = [
  "idapt config <command>",
  "",
  "  list              Show the active account's settings",
  "  get <key>         Print one setting",
  "  set <key> <value> Change one setting",
  "  unset <key>       Clear one setting",
  "  path              Print the config file location",
  "",
  `  Keys: ${[...SETTABLE].join(", ")}`,
].join("\n");

export function runConfig(args: readonly string[], io: ConfigIo): number {
  const [sub, key, value] = args;

  if (!sub || sub === "--help" || sub === "-h" || sub === "help") {
    io.print(`${CONFIG_USAGE}\n`);
    return sub ? EXIT_OK : EXIT_VALIDATION;
  }

  if (sub === "path") {
    io.print(`${configDir()}/cli-auth.json\n`);
    return EXIT_OK;
  }

  const store = loadContexts();
  const name = store.current;
  const ctx = store.contexts[name] ?? {};

  if (sub === "list") {
    const credential = ctx.refreshToken
      ? "browser sign-in (OAuth)"
      : ctx.apiKey
        ? "API key"
        : "none";
    io.print(
      [
        `account    ${name}`,
        `credential ${credential}`,
        `api-url    ${ctx.apiUrl ?? io.baseUrl}`,
        `workspace  ${ctx.defaultWorkspaceSlug ?? ctx.defaultWorkspaceId ?? "(your default workspace)"}`,
        `agent      ${ctx.defaultAgentSlug ?? ctx.defaultAgentId ?? "(none, acting as you)"}`,
        "",
      ].join("\n"),
    );
    return EXIT_OK;
  }

  if (sub === "get") {
    if (!key) {
      io.err("idapt config get <key>\n");
      return EXIT_VALIDATION;
    }
    const found = readKey(ctx, key, io);
    if (found === undefined) {
      io.err(
        `idapt config: unknown key "${key}". Keys: ${[...SETTABLE].join(", ")}\n`,
      );
      return EXIT_VALIDATION;
    }
    io.print(`${found}\n`);
    return EXIT_OK;
  }

  if (sub === "set" || sub === "unset") {
    if (!key || !SETTABLE.has(key)) {
      io.err(
        `idapt config: unknown key "${key ?? ""}". Keys: ${[...SETTABLE].join(", ")}\n`,
      );
      return EXIT_VALIDATION;
    }
    if (sub === "set" && !value) {
      io.err(`idapt config set ${key} <value>\n`);
      return EXIT_VALIDATION;
    }
    const next = { ...ctx };
    const assigned = sub === "set" ? value : undefined;
    if (key === "api-url") next.apiUrl = assigned;
    if (key === "workspace") {
      next.defaultWorkspaceId = assigned;
      next.defaultWorkspaceSlug = assigned;

      next.defaultAgentMemoryBoxId = undefined;
    }
    if (key === "agent") {
      next.defaultAgentId = assigned;
      next.defaultAgentSlug = assigned;
      next.defaultAgentMemoryBoxId = undefined;
    }
    saveContexts({ ...store, contexts: { ...store.contexts, [name]: next } });
    io.print(sub === "set" ? `Set ${key} to ${value}.\n` : `Cleared ${key}.\n`);
    return EXIT_OK;
  }

  io.err(`idapt config: unknown command "${sub}".\n\n${CONFIG_USAGE}\n`);
  return EXIT_VALIDATION;
}

function readKey(
  ctx: {
    apiUrl?: string;
    defaultWorkspaceSlug?: string;
    defaultWorkspaceId?: string;
    defaultAgentSlug?: string;
    defaultAgentId?: string;
  },
  key: string,
  io: ConfigIo,
): string | undefined {
  switch (key) {
    case "api-url":
      return ctx.apiUrl ?? io.baseUrl;
    case "workspace":
      return ctx.defaultWorkspaceSlug ?? ctx.defaultWorkspaceId ?? "";
    case "agent":
      return ctx.defaultAgentSlug ?? ctx.defaultAgentId ?? "";
    default:
      return undefined;
  }
}
