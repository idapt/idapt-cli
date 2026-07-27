

import { loadActiveCredentials, saveActiveCredentials } from "./auth/contexts";
import { executeCommand } from "./execute";
import type { AgentToolsTransport } from "./transport";

export interface AgentContextIo {
  print: (s: string) => void;
  err: (s: string) => void;
}

function pick(
  row: Record<string, unknown>,
  ...keys: string[]
): string | undefined {
  for (const k of keys) {
    const v = row[k];
    if (typeof v === "string" && v.length > 0) return v;
  }
  return undefined;
}

export async function resolveDefaultMemoryBox(
  transport: AgentToolsTransport,
): Promise<string | undefined> {
  const listed = await executeCommand(
    "agent list",
    { limit: 2 },
    { transport, mode: "json" },
  ).catch(() => null);
  if (!listed?.ok || !Array.isArray(listed.data) || listed.data.length !== 1) {
    return undefined;
  }
  return pick(
    listed.data[0] as Record<string, unknown>,
    "memory_resource_id",
    "memoryResourceId",
  );
}

export async function runAgentContext(
  sub: string,
  args: readonly string[],

  transport: AgentToolsTransport | undefined,
  io: AgentContextIo,
): Promise<number> {
  if (sub === "current") {
    const creds = loadActiveCredentials();
    if (creds.defaultAgentId) {
      const mem = creds.defaultAgentMemoryBoxId
        ? " (its Memory is your `idapt memory` target)"
        : "";
      io.print(`${creds.defaultAgentSlug ?? creds.defaultAgentId}${mem}\n`);
    } else {
      io.print(
        "(no agent selected — acting as yourself; chat + memory use your personal agent)\n",
      );
    }
    return 0;
  }

  if (sub === "clear") {
    const creds = loadActiveCredentials();
    if (!creds.defaultAgentId) {
      io.print("No agent was selected.\n");
      return 0;
    }
    const was = creds.defaultAgentSlug ?? creds.defaultAgentId;
    creds.defaultAgentId = undefined;
    creds.defaultAgentSlug = undefined;
    creds.defaultAgentMemoryBoxId = undefined;
    saveActiveCredentials(creds);
    io.print(
      `Cleared the selected agent (was ${was}). You now act as yourself.\n`,
    );
    return 0;
  }

  const ref = args.join(" ").trim();
  if (!ref) {
    io.err(
      "idapt agent use: missing agent. Usage: `idapt agent use <name | resourceId>`.\n",
    );
    return 1;
  }
  if (!transport) {
    io.err("idapt agent use: not logged in. Run `idapt login` first.\n");
    return 1;
  }

  const got = await executeCommand(
    "agent get",
    { id: ref },
    { transport, mode: "json" },
  );
  if (!got.ok) {
    io.err(
      `idapt agent use: no agent matches "${ref}" (${got.error ?? "not found"}). Try \`idapt agent list\`.\n`,
    );
    return 1;
  }
  const row = (got.data ?? {}) as Record<string, unknown>;
  const resourceId = pick(row, "id", "resourceId", "resource_id");
  if (!resourceId) {
    io.err("idapt agent use: the matched agent is missing an id.\n");
    return 1;
  }
  const name = pick(row, "name") ?? resourceId;
  const memoryBoxId = pick(row, "memory_resource_id", "memoryResourceId");

  const creds = loadActiveCredentials();
  creds.defaultAgentId = resourceId;
  creds.defaultAgentSlug = name;
  creds.defaultAgentMemoryBoxId = memoryBoxId;
  saveActiveCredentials(creds);

  const memNote = memoryBoxId
    ? " `idapt memory …` now reads/writes this agent's Memory."
    : " (this agent has no Memory box yet — memory verbs need an explicit box id.)";
  io.print(
    `Now acting as agent ${name}. \`idapt chat create\` binds it;${memNote} Override the chat agent per-call with --agent, or run \`idapt agent clear\`.\n`,
  );
  return 0;
}
