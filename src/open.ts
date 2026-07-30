

import { buildAbsoluteAppUrl } from "@shared/router/build-path";
import type { AppRoute } from "@shared/router/route-types";
import { openBrowser } from "./auth/browser";
import { EXIT_OK, EXIT_VALIDATION } from "./exit-codes";

export interface OpenIo {
  readonly print: (s: string) => void;
  readonly err: (s: string) => void;
  readonly baseUrl: string;
  readonly isTty: boolean;

  readonly open?: (url: string) => boolean;
}

const ROUTE_FOR_TYPE: Record<string, (id: string) => AppRoute> = {
  chat: (resourceId) => ({ type: "chat", resourceId }),
  chats: (resourceId) => ({ type: "chat", resourceId }),
  drive: (resourceId) => ({ type: "drive-resource", resourceId }),
  file: (resourceId) => ({ type: "drive-resource", resourceId }),
  agent: (resourceId) => ({ type: "agent-detail", resourceId }),
  agents: (resourceId) => ({ type: "agent-detail", resourceId }),
  workspace: (resourceId) => ({ type: "workspace", resourceId }),
  workspaces: (resourceId) => ({ type: "workspace", resourceId }),
  computer: (resourceId) => ({ type: "computer", resourceId }),
  computers: (resourceId) => ({ type: "computer", resourceId }),
  note: (resourceId) => ({ type: "note", resourceId }),
  notes: (resourceId) => ({ type: "note", resourceId }),
  task: (resourceId) => ({ type: "task", resourceId }),
  tasks: (resourceId) => ({ type: "task", resourceId }),
};

const OPEN_TYPES = [
  "chat",
  "drive",
  "agent",
  "workspace",
  "computer",
  "note",
  "task",
];

export const OPEN_USAGE = [
  "idapt open [<type> <id>] [--print]",
  "",
  "  idapt open                     Open the web app",
  "  idapt open chat <id>           Open a chat",
  `  Types: ${OPEN_TYPES.join(", ")}`,
].join("\n");

export function openUrlFor(
  baseUrl: string,
  args: readonly string[],
): { url: string } | { error: string } {
  const positionals = args.filter((a) => !a.startsWith("-"));

  if (positionals.length === 0) {
    return { url: buildAbsoluteAppUrl(baseUrl, { type: "new-chat" }) };
  }

  const [type, id] = positionals;
  const toRoute = ROUTE_FOR_TYPE[(type ?? "").toLowerCase()];
  if (!toRoute) {
    return {
      error: `idapt open: unknown type "${type}".\n\n${OPEN_USAGE}`,
    };
  }
  if (!id) {
    return {
      error: `idapt open: ${type} needs an id, e.g. \`idapt open ${type} <id>\`.`,
    };
  }
  try {
    return { url: buildAbsoluteAppUrl(baseUrl, toRoute(id)) };
  } catch {

    return {
      error: `idapt open: "${id}" is not a valid ${type} id.`,
    };
  }
}

export function runOpen(args: readonly string[], io: OpenIo): number {
  if (args.includes("--help") || args.includes("-h")) {
    io.print(`${OPEN_USAGE}\n`);
    return EXIT_OK;
  }
  const resolved = openUrlFor(io.baseUrl, args);
  if ("error" in resolved) {
    io.err(`${resolved.error}\n`);
    return EXIT_VALIDATION;
  }

  if (args.includes("--print") || !io.isTty) {
    io.print(`${resolved.url}\n`);
    return EXIT_OK;
  }
  const opened = (io.open ?? openBrowser)(resolved.url);
  if (!opened) io.print(`${resolved.url}\n`);
  return EXIT_OK;
}
