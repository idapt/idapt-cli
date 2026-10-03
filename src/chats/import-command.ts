

import { writeFileSync } from "node:fs";
import os from "node:os";
import { gzipSync } from "node:zlib";
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
import { oneLineTitle } from "./title";

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
    verbose?: boolean;
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
  const count =
    store.sessionCount ??
    store.projects.reduce((sum, project) => sum + project.entries.length, 0);
  return count === 1 ? "1 session" : `${count} sessions`;
}

export function summarizeSkipped(skipped: Record<string, number>): string {
  return Object.entries(skipped)
    .map(([reason, count]) => `${reason}=${count}`)
    .join(", ");
}

function progressLabel(title: string): string {
  return oneLineTitle(title, 48);
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
  const verbose = hasFlag(rest, "verbose") || ctx.globals.verbose === true;
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
  if (verbose && Object.keys(scanned.skipped).length > 0) {
    io.stderr(`  Scan ledger: ${summarizeSkipped(scanned.skipped)}\n`);
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

  io.stderr(
    `Import ${totals.count} chats (${formatBytes(totals.bytes)} of text) into ${workspace.label}?\n`,
  );
  if (io.isTty && !assumeYes) {
    const answer = (await io.prompt?.("Proceed? [Y/n]: ")) ?? "y";
    if (answer && !/^(y|yes)?$/i.test(answer.trim())) return 1;
  }

  const chatsExit = await importConversations(
    scanned.sessions,
    store.source,
    io,
    ctx,
    workspace,
    verbose,
  );
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

type ImportLedger = {
  total: number;
  imported: number;
  updated: number;
  duplicatesIgnored: number;
  modifiedIgnored: number;
  tooLarge: number;
  errors: string[];
};

function formatLedger(ledger: ImportLedger): string {
  const parts = [
    `${ledger.imported} imported`,
    `${ledger.updated} updated`,
    `${ledger.duplicatesIgnored} duplicates ignored`,
    `${ledger.modifiedIgnored} modified ignored`,
  ];
  if (ledger.tooLarge > 0) parts.push(`${ledger.tooLarge} too large`);
  parts.push(
    `${ledger.errors.length} ${ledger.errors.length === 1 ? "error" : "errors"}`,
  );
  return parts.join(", ");
}

const GZIP_THRESHOLD_BYTES = 256 * 1024;

function buildUploadBody(
  session: ParsedSession,
  source: AgentExportSource,
): Blob {
  const envelope = buildAgentChatExport({
    source,
    sessions: [session],
    generator: USER_AGENT_MARKER,
  });
  const json = JSON.stringify(envelope);
  if (json.length < GZIP_THRESHOLD_BYTES) {
    return new Blob([json], { type: "application/json" });
  }
  const gz = gzipSync(Buffer.from(json, "utf8"));
  return new Blob([new Uint8Array(gz)], { type: "application/gzip" });
}

async function importConversations(
  sessions: ParsedSession[],
  source: AgentExportSource,
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
  workspace: { id: string; label: string },
  verbose: boolean,
): Promise<number> {
  if (!ctx.token) return 2;
  const ledger: ImportLedger = {
    total: sessions.length,
    imported: 0,
    updated: 0,
    duplicatesIgnored: 0,
    modifiedIgnored: 0,
    tooLarge: 0,
    errors: [],
  };
  const headers: Record<string, string> = {
    Authorization: `Bearer ${ctx.token}`,
    "User-Agent": ctx.userAgent,
    [IDAPT_API_VERSION_HEADER]: IDAPT_API_VERSION,
  };
  io.stderr(
    `Importing ${sessions.length} chats into ${workspace.label} (one request per chat)...\n`,
  );

  for (const [index, session] of sessions.entries()) {
    let response: Response;
    try {
      const form = new FormData();
      const blob = buildUploadBody(session, source);
      form.set("file", blob, "chats-export.json");
      form.set("workspaceId", workspace.id);
      form.set("overwrite", "overwrite-if-untouched");
      response = await fetch(`${ctx.baseUrl}/api/v1/chats/import`, {
        method: "POST",
        body: form,
        headers,
      });
    } catch (err) {
      ledger.errors.push(
        `"${session.title}": ${err instanceof Error ? err.message : "network error"}`,
      );
      continue;
    }

    if (!response.ok) {
      if (response.status === 401) {
        io.stderr("Import failed: not signed in (401).\n");
        return 2;
      }
      const detail = await response.text();
      ledger.errors.push(
        `"${session.title}": HTTP ${response.status}${detail ? ` — ${detail.slice(0, 200)}` : ""}`,
      );
      continue;
    }
    if (!response.body) {
      ledger.errors.push(`"${session.title}": stream ended without events`);
      continue;
    }
    await consumeImportStream(
      response.body,
      session.title,
      ledger,
      io,
      verbose,
    );

    const done = index + 1;
    io.stderr(
      `\r${done}/${ledger.total} · ${progressLabel(session.title)}`.padEnd(120),
    );
  }
  io.stderr("\n");

  const outcome =
    ledger.errors.length === 0
      ? "completed"
      : ledger.imported + ledger.updated === 0
        ? "failed"
        : "partially completed";
  io.stdout(
    `Import ${outcome}: ${formatLedger(ledger)} (of ${ledger.total})\n`,
  );
  for (const error of ledger.errors.slice(0, 10)) {
    io.stderr(`  ! ${error}\n`);
  }
  return ledger.errors.length > 0 ? 1 : 0;
}

async function consumeImportStream(
  body: ReadableStream<Uint8Array>,
  sessionTitle: string,
  ledger: ImportLedger,
  io: ImportCommandIo,
  verbose: boolean,
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let streaming = true;
  while (streaming) {
    const { value, done } = await reader.read();
    if (value) buffer += decoder.decode(value, { stream: true });
    if (done) streaming = false;
    for (;;) {
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
      if (event.type !== "progress" || event.kind !== "conversation") continue;
      const label = progressLabel(event.label ?? sessionTitle);
      switch (event.status) {
        case "imported":
          ledger.imported++;
          break;
        case "overwritten":
          ledger.updated++;
          break;
        case "already_imported":
          ledger.duplicatesIgnored++;
          break;
        case "skipped":
          if (event.message === "changed in idapt since import; kept") {
            ledger.modifiedIgnored++;
          } else {
            ledger.tooLarge++;
            if (verbose)
              io.stderr(`\n  · ${label}: ${event.message ?? "skipped"}\n`);
          }
          break;
        case "error":
          ledger.errors.push(`"${label}": ${event.message ?? "error"}`);
          break;
        default:
          ledger.modifiedIgnored++;
      }
    }
  }
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
