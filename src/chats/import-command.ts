

import { writeFileSync } from "node:fs";
import os from "node:os";
import { IDAPT_API_VERSION, IDAPT_API_VERSION_HEADER } from "@idapt/sdk";
import type {
  AgentChatExport,
  AgentExportSource,
} from "@shared/chat/agent-export";
import { execute } from "../execute";
import { formatBytes } from "../progress";
import { selectMany, selectOne } from "../select";
import { createFetchTransport } from "../transport";
import { resolveWorkspaceRef } from "../workspace-context";
import {
  type DetectedAgentStore,
  detectAgentStores,
  detectedStoreFor,
} from "./detect";
import type { ParsedSession } from "./readers/claude-code";
import {
  buildAgentChatExport,
  readSessionsForScope,
  sessionsTotals,
} from "./scan";
import { type DetectedSkill, detectSkills } from "./skills-scan";

export type ImportCommandIo = {
  argv: readonly string[];
  env: Record<string, string | undefined>;
  stdout: (s: string) => void;
  stderr: (s: string) => void;
  isTty: boolean;
  prompt?: (question: string) => Promise<string>;
};

export type ImportCommandCtx = {
  baseUrl: string;
  token?: string;
  userAgent: string;

  globals: {
    workspace?: string;
    all?: boolean;
    yes?: boolean;
  };
};

function flagValue(rest: readonly string[], name: string): string | undefined {
  const index = rest.indexOf(`--${name}`);
  return index >= 0 ? rest[index + 1] : undefined;
}

function hasFlag(rest: readonly string[], name: string): boolean {
  return rest.includes(`--${name}`);
}

function homedir(): string {
  return process.env.IDAPT_AGENT_STORES_ROOT ?? os.homedir();
}

function sessionsWord(store: DetectedAgentStore): string {
  const count = store.projects.reduce(
    (sum, project) => sum + project.entries.length,
    0,
  );
  return count === 1 ? "1 session" : `${count} sessions`;
}

export function summarizeSkipped(skipped: Record<string, number>): string {
  return Object.entries(skipped)
    .map(([reason, count]) => `${reason}=${count}`)
    .join(", ");
}

function formatDate(date: Date): string {
  return date.toISOString().slice(0, 16).replace("T", " ");
}

export async function runChatsImport(
  rest: readonly string[],
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
): Promise<number> {
  const from = flagValue(rest, "from") as AgentExportSource | undefined;
  const outFile = flagValue(rest, "out");
  const dryRun = hasFlag(rest, "dry-run") || hasFlag(rest, "dry_run");
  const skipSkills = hasFlag(rest, "no-skills") || hasFlag(rest, "no_skills");
  const scopeAll = hasFlag(rest, "all") || ctx.globals.all === true;
  const projectArg = flagValue(rest, "project");
  const workspaceFlag =
    flagValue(rest, "workspace-id") ??
    ctx.globals.workspace ??
    io.env.IDAPT_WORKSPACE;
  const assumeYes = ctx.globals.yes === true;

  if (!io.isTty && !outFile && (!workspaceFlag || !from)) {
    io.stderr(
      "idapt chat import: non-interactive runs need --from and --workspace.\n" +
        "  Example: idapt chat import --from claude-code --workspace my-team --yes\n",
    );
    return 6;
  }

  const stores = detectAgentStores({ home: homedir(), env: io.env });
  let store = from ? detectedStoreFor(stores, from) : undefined;
  if (!store) {
    const candidates = stores.filter((store) => store.installed);
    if (candidates.length === 0) {
      io.stderr(
        "No coding-agent session stores found.\n" +
          "  Looked for Claude Code (~/.claude/projects), Codex (~/.codex/sessions),\n" +
          "  OpenCode (~/.local/share/opencode).\n",
      );
      return 6;
    }
    if (!io.isTty) {
      io.stderr(
        `Multiple agents installed; pass --from (${candidates.map((c) => c.source).join(", ")}).\n`,
      );
      return 6;
    }
    const picked = await selectOne(
      io,
      "Which agent's chats do you want to import?",
      candidates.map((candidate) => ({
        label: candidate.label,
        hint: `${sessionsWord(candidate)} · ${candidate.root ?? ""}`,
      })),
    );
    if (picked === null) return 1;
    store = candidates[picked];
  }
  if (!store?.installed) {
    io.stderr(
      `${store?.label ?? "The agent"} is not detected on this machine (${store?.reason ?? "no session stores"}).\n`,
    );
    return 6;
  }

  const scope = projectArg ?? (scopeAll ? undefined : process.cwd());
  io.stderr(
    `Scanning ${store.label} sessions in ${scope ?? "all projects on this machine"}…\n`,
  );
  const scanned = await readSessionsForScope(store, scope);
  if (scanned.sessions.length === 0) {
    io.stderr(
      `No importable ${store.label} sessions found in scope.\n` +
        (Object.keys(scanned.skipped).length
          ? `  Skipped: ${summarizeSkipped(scanned.skipped)}\n`
          : "") +
        "  Hint: use --all to scan every project on this machine.\n",
    );
    return 6;
  }

  const totals = sessionsTotals(scanned.sessions);
  if (outFile) {
    const envelope = buildAgentChatExport({
      source: store.source,
      sessions: scanned.sessions,
      generator: USER_AGENT_MARKER,
    });
    writeExportFile(outFile, envelope);
    io.stdout(
      `Wrote ${totals.count} chats (${formatBytes(totals.bytes)} of text) to ${outFile}\n`,
    );
    return 0;
  }

  const workspace = await resolveWorkspaceTarget(workspaceFlag, io, ctx);
  if (!workspace) return 6;

  if (dryRun) {
    io.stdout(
      `Would import ${totals.count} chats (${formatBytes(totals.bytes)} of text) into ${workspace.label}\n`,
    );
    return 0;
  }

  const selected = io.isTty
    ? await pickConversations(scanned.sessions, io)
    : scanned.sessions;
  if (selected.length === 0) return 1;

  const selectedTotals = sessionsTotals(selected);
  io.stderr(
    `Import ${selectedTotals.count} chats (${formatBytes(selectedTotals.bytes)} of text) into ${workspace.label}?\n`,
  );
  if (io.isTty && !assumeYes) {
    const answer = (await io.prompt?.("Proceed? [Y/n]: ")) ?? "y";
    if (answer && !/^(y|yes)?$/i.test(answer.trim())) return 1;
  }

  const envelope = buildAgentChatExport({
    source: store.source,
    sessions: selected,
    generator: USER_AGENT_MARKER,
  });
  const chatsExit = await importEnvelope(envelope, io, ctx, workspace);
  if (chatsExit !== 0) return chatsExit;

  if (store.source === "claude-code" && !skipSkills) {
    return importSkills(io, ctx, workspace);
  }
  return 0;
}

const USER_AGENT_MARKER = "idapt-cli";

async function resolveWorkspaceTarget(
  workspaceFlag: string | undefined,
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
): Promise<{ id: string; label: string } | null> {
  if (!ctx.token) return null;
  const transport = createFetchTransport({
    baseUrl: ctx.baseUrl,
    token: ctx.token,
    userAgent: ctx.userAgent,
  });

  if (workspaceFlag) {
    const resolved = await resolveWorkspaceRef(transport, workspaceFlag);
    if (resolved.ok) return { id: resolved.resourceId, label: resolved.label };
    io.stderr(`${resolved.error}\n`);
    return null;
  }

  const listed = await execute("idapt workspace list", {
    transport,
    mode: "json",
  });
  if (!listed.ok) {
    io.stderr(`${listed.error ?? "could not list workspaces"}\n`);
    return null;
  }
  const rows = Array.isArray(listed.data) ? listed.data : [];
  const picked = await selectOne(
    io,
    "Import into which workspace?",
    rows.map((row) => ({
      label: String(row.name ?? row.slug ?? row.id ?? "workspace"),
      hint: String(row.slug ?? ""),
    })),
  );
  if (picked === null) return null;
  const row = rows[picked];
  const label = String(row.name ?? row.slug ?? "workspace");
  const id = String(row.id ?? row.resourceId ?? "");
  if (!id) {
    io.stderr(
      `Workspace ${label} has no usable id; pass --workspace explicitly.\n`,
    );
    return null;
  }
  return { id, label };
}

async function pickConversations(
  sessions: ParsedSession[],
  io: ImportCommandIo,
): Promise<ParsedSession[]> {
  const options = sessions.map((session) => ({
    label: session.title,
    hint: `${formatDate(session.updatedAt)} · ${formatBytes(sessionsTotals([session]).bytes)}`,
  }));
  const picked = await selectMany(io, "Chats to import:", options, true);
  if (picked === null) return [];
  return picked.map((index) => sessions[index]);
}

function writeExportFile(path_: string, envelope: AgentChatExport): void {
  writeFileSync(path_, JSON.stringify(envelope, null, 1), { mode: 0o600 });
}

interface ImportApiEvent {
  type: "started" | "progress" | "complete";
  kind?: string;
  totalConversations?: number;
  label?: string;
  status?: string;
  message?: string;
  outcome?: string;
  imported?: number;
  alreadyImported?: number;
  overwritten?: number;
  skipped?: number;
  errors?: string[];
  errorMessage?: string;
}

async function importEnvelope(
  envelope: AgentChatExport,
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
  workspace: { id: string; label: string },
): Promise<number> {
  if (!ctx.token) return 2;
  const form = new FormData();
  const blob = new Blob([JSON.stringify(envelope)], {
    type: "application/json",
  });
  form.set("file", blob, "chats-export.json");
  form.set("workspaceId", workspace.id);
  form.set("overwrite", "overwrite-if-untouched");

  let response: Response;
  try {
    response = await fetch(`${ctx.baseUrl}/api/v1/chats/import`, {
      method: "POST",
      body: form,
      headers: {
        Authorization: `Bearer ${ctx.token}`,
        "User-Agent": ctx.userAgent,
        [IDAPT_API_VERSION_HEADER]: IDAPT_API_VERSION,
      },
    });
  } catch (err) {
    io.stderr(
      `Import request failed: ${err instanceof Error ? err.message : "network error"}\n`,
    );
    return 1;
  }

  if (!response.ok) {
    io.stderr(`Import failed (${response.status}): ${await response.text()}\n`);
    return response.status === 401 ? 2 : 1;
  }

  if (!response.body) {
    io.stderr("Import stream ended without events.\n");
    return 1;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let imported = 0;
  let overwritten = 0;
  let alreadyImported = 0;
  let skipped = 0;
  const errors: string[] = [];
  let lastLabel = "";
  let done = false;

  while (!done) {
    const { value, done: streamDone } = await reader.read();
    if (value) buffer += decoder.decode(value, { stream: true });
    if (streamDone) done = true;
    while (true) {
      const newline = buffer.indexOf("\n");
      if (newline < 0) break;
      const line = buffer.slice(0, newline).trim();
      buffer = buffer.slice(newline + 1);
      if (!line) continue;
      let event: ImportApiEvent;
      try {
        event = JSON.parse(line) as ImportApiEvent;
      } catch {
        continue;
      }
      if (event.type === "started") {
        io.stderr(
          `Importing ${event.totalConversations ?? "?"} chats into ${workspace.label}...\n`,
        );
      }
      if (event.type === "progress" && event.kind === "conversation") {
        if (event.status === "imported") imported++;
        else if (event.status === "overwritten") overwritten++;
        else if (event.status === "already_imported") alreadyImported++;
        else if (event.status === "error")
          errors.push(event.message ?? "error");
        else skipped++;
        lastLabel = event.label ?? "";
        io.stderr(
          `\r${imported + overwritten} imported, ${alreadyImported} kept, ${skipped} skipped, ${errors.length} errors · ${lastLabel}`.padEnd(
            140,
          ),
        );
      }
      if (event.type === "complete") {
        io.stderr("\n");
        io.stdout(
          `Import ${event.outcome}: ${event.imported ?? 0} imported, ${event.overwritten ?? 0} overwritten, ` +
            `${event.alreadyImported ?? 0} already imported, ${event.skipped ?? 0} skipped, ` +
            `${(event.errors ?? []).length} errors\n`,
        );
        for (const error of (event.errors ?? []).slice(0, 10)) {
          io.stderr(`  ! ${error}\n`);
        }
        return event.outcome === "completed" ? 0 : 1;
      }
    }
  }
  io.stderr("Import stream ended without a complete event.\n");
  return 1;
}

async function pickSkills(
  skills: DetectedSkill[],
  io: ImportCommandIo,
): Promise<DetectedSkill[]> {
  const options = skills.map((skill) => ({
    label: skill.name,
    hint: skill.description
      ? skill.description.length > 60
        ? `${skill.description.slice(0, 57)}...`
        : skill.description
      : "SKILL.md",
  }));
  const picked = await selectMany(io, "Skills to import:", options, true);
  if (picked === null) return [];
  return picked.map((index) => skills[index]);
}

async function importSkills(
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
  workspace: { id: string; label: string },
): Promise<number> {
  if (!ctx.token) return 2;
  const skills = detectSkills({
    home: homedir(),
    cwd: process.cwd(),
    env: io.env,
  });
  if (skills.length === 0) return 0;

  const selected = io.isTty ? await pickSkills(skills, io) : skills;
  if (selected.length === 0) return 0;

  const authHeaders: Record<string, string> = {
    Authorization: `Bearer ${ctx.token}`,
    "User-Agent": ctx.userAgent,
    [IDAPT_API_VERSION_HEADER]: IDAPT_API_VERSION,
  };

  const existing = new Set<string>();
  try {
    const list = await fetch(`${ctx.baseUrl}/api/v1/skills?limit=100`, {
      headers: authHeaders,
    });
    if (list.ok) {
      const body = (await list.json()) as {
        data?: Array<{ name?: string; slug?: string }>;
      };
      for (const row of body.data ?? []) {
        if (row.name) existing.add(row.name.toLowerCase());
        if (row.slug) existing.add(row.slug.toLowerCase());
      }
    }
  } catch {

  }

  io.stderr(`Importing ${selected.length} skills into ${workspace.label}...\n`);
  let created = 0;
  let kept = 0;
  let failed = 0;
  for (const skill of selected) {
    const key = skill.name.toLowerCase();
    if (existing.has(key)) {
      kept++;
      io.stderr(`  ${skill.name}: already exists, kept\n`);
      continue;
    }
    try {
      const res = await fetch(`${ctx.baseUrl}/api/v1/skills`, {
        method: "POST",
        headers: { ...authHeaders, "content-type": "application/json" },
        body: JSON.stringify({
          name: skill.name,
          description: skill.description ?? null,
          body: skill.body,
          workspace_id: workspace.id,
        }),
      });
      if (res.ok) {
        created++;
      } else if (res.status === 409) {
        kept++;
        io.stderr(`  ${skill.name}: already exists, kept\n`);
      } else {
        failed++;
        io.stderr(`  ! ${skill.name}: ${res.status}\n`);
      }
    } catch {
      failed++;
      io.stderr(`  ! ${skill.name}: network error\n`);
    }
  }
  io.stdout(`Skills: ${created} imported, ${kept} kept, ${failed} errors\n`);
  return failed > 0 ? 1 : 0;
}
