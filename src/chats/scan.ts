

import path from "node:path";
import {
  AGENT_EXPORT_SCHEMA,
  type AgentChatExport,
  type AgentExportSource,
} from "@shared/chat/agent-export";
import type { DetectedAgentStore } from "./detect";
import type { ParsedSession } from "./readers/claude-code";
import { readClaudeCodeProject } from "./readers/claude-code";
import { readCodexRollout } from "./readers/codex";
import { readOpenCodeDatabase } from "./readers/opencode";

export type ScannedSessions = {
  sessions: ParsedSession[];

  skipped: Record<string, number>;
};

export function readSessionsForScope(
  store: DetectedAgentStore,
  projectFilter?: string,
): Promise<ScannedSessions> {
  return readSessionsForStore(store, projectFilter);
}

export async function readSessionsForStore(
  store: DetectedAgentStore,
  projectFilter?: string,
): Promise<ScannedSessions> {
  const skipped: Record<string, number> = {};
  const addSkip = (reason: string): void => {
    skipped[reason] = (skipped[reason] ?? 0) + 1;
  };
  const sessions: ParsedSession[] = [];

  if (store.source === "claude-code") {
    for (const project of store.projects) {
      if (!projectMatches(project, projectFilter)) continue;
      const parsed = await readClaudeCodeProject(project);
      for (const session of parsed) {
        if (!sessionMatches(session, projectFilter)) {
          addSkip("outside-scope");
          continue;
        }
        sessions.push(session);
      }
    }
    return { sessions, skipped };
  }

  if (store.source === "codex") {
    for (const project of store.projects) {
      for (const file of project.entries) {
        const parsed = await readCodexRollout(file);
        if (!parsed) {
          addSkip("no-convertible-messages");
          continue;
        }
        if (!sessionMatches(parsed, projectFilter)) {
          addSkip("outside-scope");
          continue;
        }
        sessions.push(parsed);
      }
    }
    return { sessions, skipped };
  }

  const parsedAll = await readOpenCodeDatabase(store.root as string);
  for (const session of parsedAll) {
    if (!sessionMatches(session, projectFilter)) {
      addSkip("outside-scope");
      continue;
    }
    sessions.push(session);
  }
  return { sessions, skipped };
}

function projectMatches(
  project: { path: string },
  projectFilter?: string,
): boolean {
  if (!projectFilter) return true;
  if (project.path === projectFilter) return true;

  const encoded = projectFilter.split(path.sep).filter(Boolean).join("-");
  return project.path.includes(encoded);
}

function sessionMatches(
  session: { project?: string },
  projectFilter?: string,
): boolean {
  if (!projectFilter) return true;
  const cwd = session.project;
  if (!cwd) return false;
  const rel = path.relative(projectFilter, cwd);
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

export function bestProjectForScope(
  store: DetectedAgentStore,
  scopeDir: string,
): string | undefined {
  if (store.source === "claude-code") {
    for (const project of store.projects) {
      if (projectMatches(project, scopeDir)) return scopeDir;
    }
    return undefined;
  }
  return scopeDir;
}

export function buildAgentChatExport(params: {
  source: AgentExportSource;
  sessions: ParsedSession[];
  generator: string;
  exportedAt?: Date;
}): AgentChatExport {
  return {
    schema: AGENT_EXPORT_SCHEMA,
    source: params.source,
    generator: params.generator,
    exportedAt: params.exportedAt ?? new Date(),
    conversations: params.sessions.map((session) => ({
      sourceId: session.sourceId,
      title: session.title,
      ...(session.project ? { project: session.project } : {}),
      createdAt: session.createdAt,
      updatedAt: session.updatedAt,
      messages: session.messages,
    })),
  };
}

export function sessionSizeBytes(session: ParsedSession): number {
  let total = 0;
  for (const message of session.messages) {
    total += message.text?.length ?? 0;
    total += message.reasoning?.length ?? 0;
    total += message.toolResult?.length ?? 0;
    total += message.toolCalls
      ? message.toolCalls.reduce(
          (sum, call) => sum + JSON.stringify(call.input ?? {}).length,
          0,
        )
      : 0;
  }
  return total;
}

export function sessionsTotals(sessions: ParsedSession[]): {
  count: number;
  bytes: number;
} {
  let bytes = 0;
  for (const session of sessions) bytes += sessionSizeBytes(session);
  return { count: sessions.length, bytes };
}
