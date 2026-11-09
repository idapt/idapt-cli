

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
  sessionSizeBytes,
  sessionsTotals,
} from "./scan";
import { type DetectedSkill, detectSkills } from "./skills-scan";
import { oneLineTitle } from "./title";

function terminalWidth(): number {
  return process.stdout.columns || 120;
}

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

const ALL_SOURCES = "all";

export function progressLine(params: {
  done: number;
  total: number;
  bytesDone: number;
  bytesTotal: number;
  label: string;
  width?: number;
}): string {
  const width = params.width ?? 120;
  const pct =
    params.bytesTotal > 0
      ? Math.round((params.bytesDone / params.bytesTotal) * 100)
      : params.done >= params.total
        ? 100
        : 0;
  const head = `${params.done}/${params.total} · ${pct}% · ${formatBytes(params.bytesDone)}/${formatBytes(params.bytesTotal)}`;
  const label = ` · ${params.label}`;
  return `${`\r${head}${label}`.slice(0, width)}\x1b[K`;
}

export async function runChatsImport(
  rest: readonly string[],
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
): Promise<number> {
  const from = flagValue(rest, "from") as AgentExportSource | "all" | undefined;
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
  const candidates = stores.filter((s) => s.installed);

  let scope = projectArg ?? (scopeAll ? undefined : process.cwd());
  if (io.isTty && !projectArg && !scopeAll) {
    const picked = await selectOne(
      io,
      "Import chats from just this folder, or every project on this computer?",
      [
        { label: "Only this folder", hint: process.cwd() },
        {
          label: "All projects on this computer",
          hint: "the whole machine's stores",
        },
      ],
    );
    if (picked === null) return 1;
    scope = picked === 0 ? process.cwd() : undefined;
  }

  let groups: ScanGroup[];
  if (from === ALL_SOURCES) {
    if (candidates.length === 0) return noStoresFound(io);
    groups = await scanGroups(candidates, scope, io, verbose);
  } else if (from) {
    const store = detectedStoreFor(stores, from);
    if (!store?.installed) {
      io.stderr(
        `${store?.label ?? "The agent"} is not detected on this machine (${store?.reason ?? "no session stores"}).\n`,
      );
      return 6;
    }
    groups = await scanGroups([store], scope, io, verbose);
  } else {
    if (candidates.length === 0) return noStoresFound(io);
    if (!io.isTty) {
      io.stderr(
        `Multiple agents installed; pass --from (${candidates.map((c) => c.source).join(", ")}, or all).\n`,
      );
      return 6;
    }
    if (candidates.length === 1) {
      groups = await scanGroups([candidates[0]], scope, io, verbose);
    } else {
      const picked = await selectOne(
        io,
        "Which agent's chats do you want to import?",
        sourcePickerOptions(candidates),
      );
      if (picked === null) return 1;
      groups =
        picked === 0
          ? await scanGroups(candidates, scope, io, verbose)
          : await scanGroups([candidates[picked - 1]], scope, io, verbose);
    }
  }

  const sessions = groups.flatMap((group) => group.sessions);
  if (sessions.length === 0) {
    const skipped = mergeSkipped(groups);
    io.stderr(
      "No importable sessions found in scope.\n" +
        (Object.keys(skipped).length
          ? `  Skipped: ${summarizeSkipped(skipped)}\n`
          : "") +
        "  Hint: choose \u201cAll projects on this computer\u201d or pass --all.\n",
    );
    return 6;
  }

  const totals = sessionsTotals(sessions);
  if (outFile) {

    for (const group of groups) {
      const suffix = groups.length > 1 ? `-${group.source}` : "";
      const groupPath = outFile.replace(/\.json$/i, `${suffix}.json`);
      const envelope = buildAgentChatExport({
        source: group.source,
        sessions: group.sessions,
        generator: USER_AGENT_MARKER,
      });
      writeExportFile(groupPath, envelope);
      io.stdout(`Wrote ${group.sessions.length} chats to ${groupPath}\n`);
    }
    io.stdout(
      `${totals.count} chats (${formatBytes(totals.bytes)} of text) in total\n`,
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

  const sourceWord =
    groups.length === 1 ? groups[0].label : `${groups.length} coding agents`;
  io.stderr(
    `Import ${totals.count} chats (${formatBytes(totals.bytes)} of text) from ${sourceWord} into ${workspace.label}?\n`,
  );
  if (io.isTty && !assumeYes) {
    const answer = (await io.prompt?.("Proceed? [Y/n]: ")) ?? "y";
    if (answer && !/^(y|yes)?$/i.test(answer.trim())) return 1;
  }

  const chatsExit = await importConversations(
    groups,
    io,
    ctx,
    workspace,
    verbose,
  );
  if (chatsExit !== 0) return chatsExit;

  if (groups.some((group) => group.source === "claude-code") && !skipSkills) {
    return importSkills(io, ctx, workspace);
  }
  return 0;
}

function noStoresFound(io: ImportCommandIo): number {
  io.stderr(
    "No coding-agent session stores found.\n" +
      "  Looked for Claude Code (~/.claude/projects), Codex (~/.codex/sessions),\n" +
      "  OpenCode (~/.local/share/opencode).\n",
  );
  return 6;
}

type ScanGroup = {
  source: AgentExportSource;
  label: string;
  sessions: ParsedSession[];
  skipped: Record<string, number>;
};

export function sourcePickerOptions(
  candidates: DetectedAgentStore[],
): Array<{ label: string; hint: string }> {
  const total = candidates.reduce(
    (sum, store) =>
      sum +
      (store.sessionCount ??
        store.projects.reduce((s, project) => s + project.entries.length, 0)),
    0,
  );
  return [
    {
      label: "All coding agents",
      hint: `${total === 1 ? "1 session" : `${total} sessions`} · every harness on this machine`,
    },
    ...candidates.map((candidate) => ({
      label: candidate.label,
      hint: `${sessionsWord(candidate)} · ${candidate.root ?? ""}`,
    })),
  ];
}

async function scanGroups(
  stores: DetectedAgentStore[],
  scope: string | undefined,
  io: ImportCommandIo,
  verbose: boolean,
): Promise<ScanGroup[]> {
  const groups: ScanGroup[] = [];
  for (const store of stores) {
    io.stderr(
      `Scanning ${store.label} sessions in ${scope ?? "all projects on this machine"}…\n`,
    );
    const scanned = await readSessionsForScope(store, scope);
    if (verbose && Object.keys(scanned.skipped).length > 0) {
      io.stderr(`  Scan ledger: ${summarizeSkipped(scanned.skipped)}\n`);
    }
    groups.push({
      source: store.source,
      label: store.label,
      sessions: scanned.sessions,
      skipped: scanned.skipped,
    });
  }
  return groups;
}

function mergeSkipped(groups: ScanGroup[]): Record<string, number> {
  const merged: Record<string, number> = {};
  for (const group of groups) {
    for (const [reason, count] of Object.entries(group.skipped)) {
      merged[reason] = (merged[reason] ?? 0) + count;
    }
  }
  return merged;
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
  groups: ScanGroup[],
  io: ImportCommandIo,
  ctx: ImportCommandCtx,
  workspace: { id: string; label: string },
  verbose: boolean,
): Promise<number> {
  if (!ctx.token) return 2;
  const units = groups.flatMap((group) =>
    group.sessions.map((session) => ({
      session,
      source: group.source,
      bytes: sessionSizeBytes(session),
    })),
  );
  const bytesTotal = units.reduce((sum, unit) => sum + unit.bytes, 0);
  const ledger: ImportLedger = {
    total: units.length,
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
    `Importing ${units.length} chats into ${workspace.label} (one request per chat)...\n`,
  );

  let bytesDone = 0;
  for (const [index, unit] of units.entries()) {
    const session = unit.session;
    let response: Response;
    try {
      const form = new FormData();
      const blob = buildUploadBody(session, unit.source);
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
        io.stderr("\nImport failed: not signed in (401).\n");
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

    bytesDone += unit.bytes;
    if (io.isTty) {
      io.stderr(
        progressLine({
          done: index + 1,
          total: ledger.total,
          bytesDone,
          bytesTotal,
          label: progressLabel(session.title),
          width: terminalWidth(),
        }),
      );
    }
  }
  if (io.isTty) io.stderr("\n");

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

  let pack: { id: string } | null = null;
  try {
    const list = await fetch(`${ctx.baseUrl}/api/v1/skill-packs`, {
      headers: authHeaders,
    });
    if (list.ok) {
      const body = (await list.json()) as {
        data?: Array<{ id?: string; slug?: string; scope?: string }>;
      };
      const row = (body.data ?? []).find(
        (candidate) =>
          candidate.scope === "custom" && candidate.slug === "workspace-skills",
      );
      pack = row?.id ? { id: row.id } : null;
    }
  } catch {

  }
  if (!pack?.id) {
    io.stderr(`  ! No workspace skills pack found in ${workspace.label}\n`);
    return 1;
  }

  const existing = new Set<string>();
  try {
    const res = await fetch(
      `${ctx.baseUrl}/api/v1/skill-packs/${pack.id}/skills?limit=200`,
      { headers: authHeaders },
    );
    if (res.ok) {
      const body = (await res.json()) as {
        data?: Array<{ slug?: string }>;
      };
      for (const row of body.data ?? []) {
        if (row.slug) existing.add(row.slug);
      }
    }
  } catch {

  }

  io.stderr(`Importing ${selected.length} skills into ${workspace.label}...\n`);
  let created = 0;
  let kept = 0;
  let failed = 0;
  for (const skill of selected) {
    const slug = kebabSlug(skill.folder || skill.name);
    if (!slug) {
      failed++;
      io.stderr(`  ! ${skill.name}: no usable folder name\n`);
      continue;
    }
    if (existing.has(slug)) {
      kept++;
      io.stderr(`  ${skill.name}: already exists, kept\n`);
      continue;
    }
    const content = [
      "---",
      `name: ${slug}`,
      `title: ${JSON.stringify(skill.name)}`,
      `description: ${JSON.stringify(skill.description ?? "")}`,
      "---",
      "",
      skill.body.trim(),
      "",
    ].join("\n");
    try {
      const res = await fetch(
        `${ctx.baseUrl}/api/v1/skill-packs/${pack.id}/file`,
        {
          method: "POST",
          headers: { ...authHeaders, "content-type": "application/json" },
          body: JSON.stringify({ path: `${slug}/SKILL.md`, content }),
        },
      );
      if (res.ok) {
        created++;
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

function kebabSlug(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
