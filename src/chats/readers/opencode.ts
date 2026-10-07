

import type {
  AgentExportMessage,
  AgentExportToolCall,
} from "@shared/chat/agent-export";
import { getSqliteDatabaseSync } from "../sqlite";
import { deriveSessionTitle } from "../title";
import type { ParsedSession } from "./claude-code";

type Json = Record<string, unknown>;
type DatabaseSync = InstanceType<ReturnType<typeof getSqliteDatabaseSync>>;

function rec(value: unknown): Json | null {
  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return rec(parsed);
    } catch {
      return null;
    }
  }
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Json)
    : null;
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function fromUnixMs(value: unknown): Date {
  return new Date(
    typeof value === "number" && Number.isFinite(value) ? value : 0,
  );
}

type SessionRow = {
  id: string;
  directory: string | null;
  title: string | null;
  slug: string | null;
  parent_id: string | null;
  time_created: number | null;
  time_updated: number | null;
};

function hasModernSchema(db: DatabaseSync): boolean {
  const rows = db
    .prepare(
      "SELECT name FROM sqlite_master WHERE type='table' AND name IN ('session','message','part')",
    )
    .all() as Array<{ name: string }>;
  const names = new Set(rows.map((row) => row.name));
  return names.size === 3;
}

export async function readOpenCodeDatabase(
  dbPath: string,
): Promise<ParsedSession[]> {
  const DatabaseSync = getSqliteDatabaseSync();
  const db = new DatabaseSync(dbPath, { readOnly: true });
  try {
    return hasModernSchema(db) ? readModernSchema(db) : readLegacySchema(db);
  } finally {
    db.close();
  }
}

export function countOpenCodeSessions(dbPath: string): number | undefined {
  const DatabaseSync = getSqliteDatabaseSync();
  let db: DatabaseSync;
  try {
    db = new DatabaseSync(dbPath, { readOnly: true });
  } catch {
    return undefined;
  }
  try {
    const roots = hasModernSchema(db)
      ? (db
          .prepare("SELECT COUNT(*) AS n FROM session WHERE parent_id IS NULL")
          .get() as { n: number } | undefined)
      : (db
          .prepare(
            "SELECT COUNT(*) AS n FROM session_v2 WHERE parent_id IS NULL",
          )
          .get() as { n: number } | undefined);
    return roots && Number.isFinite(roots.n) ? roots.n : undefined;
  } catch {
    return undefined;
  } finally {
    db.close();
  }
}

function readModernSchema(db: DatabaseSync): ParsedSession[] {
  const sessions: ParsedSession[] = [];
  const skipCount = new Map<string, number>();
  const skip = (reason: string, n = 1): void => {
    skipCount.set(reason, (skipCount.get(reason) ?? 0) + n);
  };

  const sessionRows = db
    .prepare(
      "SELECT id, directory, title, slug, parent_id, time_created, time_updated FROM session WHERE parent_id IS NULL ORDER BY time_created, id",
    )
    .all() as SessionRow[];
  const partStmt = db.prepare(
    "SELECT data FROM part WHERE message_id = ? ORDER BY time_created, id",
  );

  for (const row of sessionRows) {
    const parsed: ParsedSession = {
      sourceId: row.id,
      title: str(row.title) || str(row.slug) || "Untitled session",
      project: str(row.directory) || undefined,
      createdAt: fromUnixMs(row.time_created),
      updatedAt: fromUnixMs(row.time_updated),
      messages: [],
      skipped: {},
    };

    const messageRows = db
      .prepare(
        "SELECT id, time_created, data FROM message WHERE session_id = ? ORDER BY time_created, id",
      )
      .all(row.id) as Array<{
      id: string;
      time_created: number | null;
      data: string | null;
    }>;

    for (const messageRow of messageRows) {
      const data = rec(messageRow.data ?? "");
      if (!data) {
        parsed.skipped["malformed-json"] =
          (parsed.skipped["malformed-json"] ?? 0) + 1;
        continue;
      }
      const createdAt = fromUnixMs(messageRow.time_created);

      const parts = (
        partStmt.all(messageRow.id) as Array<{ data: string | null }>
      )
        .map((partRow) => rec(partRow.data ?? ""))
        .filter((part): part is Json => part !== null);

      if (str(data.role) === "user") {
        const texts: string[] = [];
        for (const part of parts) {
          if (str(part.type) === "text") {
            const text = str(part.text).trim();
            if (text) texts.push(text);
          } else if (str(part.type) === "file") {
            parsed.skipped.attachments = (parsed.skipped.attachments ?? 0) + 1;
          }
        }
        const text = texts.join("\n\n").trim();
        if (text)
          parsed.messages.push({
            role: "user",
            text,
            createdAt: createdAt ?? undefined,
          });
        else skip("empty-message");
        continue;
      }

      if (str(data.role) !== "assistant") {
        skip(`unknown-role:${str(data.role) || "none"}`);
        continue;
      }

      const step = assistantFromParts(parts, skip);

      const modelSlug = str(data.modelID) || str(rec(data.model)?.id);
      if (step) {
        step.message.createdAt = createdAt ?? undefined;
        step.message.modelSlug = modelSlug || undefined;
        for (const result of step.results)
          result.createdAt = createdAt ?? undefined;
        parsed.messages.push(step.message, ...step.results);
      } else {

        const error = rec(data.error);
        const errorMessage = str(error?.message).trim();
        if (errorMessage)
          parsed.messages.push({
            role: "assistant",
            text: errorMessage,
            createdAt: createdAt ?? undefined,
          });
      }
    }

    if (parsed.messages.length > 0) {
      parsed.title = deriveSessionTitle(
        parsed.messages,
        str(row.title) || str(row.slug) || null,
      );
      sessions.push(parsed);
    } else {
      skip("no-convertible-messages");
    }
  }

  foldStoreSkips(sessions, skipCount);
  return sessions;
}

function readLegacySchema(db: DatabaseSync): ParsedSession[] {
  const sessions: ParsedSession[] = [];
  const skipCount = new Map<string, number>();
  const skip = (reason: string, n = 1): void => {
    skipCount.set(reason, (skipCount.get(reason) ?? 0) + n);
  };

  const sessionRows = db
    .prepare(
      "SELECT id, directory, title, slug, parent_id, time_created, time_updated FROM session_v2 WHERE parent_id IS NULL ORDER BY time_created",
    )
    .all() as SessionRow[];

  const messageStmt = db.prepare(
    "SELECT id, type, seq, time_created, time_updated, data FROM session_message WHERE session_id = ? ORDER BY seq",
  );

  for (const row of sessionRows) {
    const parsed: ParsedSession = {
      sourceId: row.id,
      title: (str(row.title) || str(row.slug) || "Untitled session").slice(
        0,
        200,
      ),
      project: str(row.directory) || undefined,
      createdAt: fromUnixMs(row.time_created),
      updatedAt: fromUnixMs(row.time_updated),
      messages: [],
      skipped: {},
    };

    const messageRows = messageStmt.all(row.id) as Array<{
      id: string;
      type: string | null;
      seq: number | null;
      time_created: number | null;
      time_updated: number | null;
      data: string | null;
    }>;

    for (const message of messageRows) {
      const data = rec(message.data ?? "");
      if (!data) {
        parsed.skipped["malformed-json"] =
          (parsed.skipped["malformed-json"] ?? 0) + 1;
        continue;
      }
      const type = str(message.type);
      const createdAt = fromUnixMs(message.time_created);

      if (type === "user") {
        const text = str(data.text).trim();
        if (text)
          parsed.messages.push({
            role: "user",
            text,
            createdAt: createdAt ?? undefined,
          });
        else skip("empty-message");
        if (Array.isArray(data.files) && data.files.length > 0) {
          parsed.skipped.attachments =
            (parsed.skipped.attachments ?? 0) + data.files.length;
        }
        continue;
      }

      if (type === "shell") {
        const command = str(data.command).trim();
        if (command)
          parsed.messages.push({
            role: "user",
            text: `! ${command}`,
            createdAt: createdAt ?? undefined,
          });
        const output = rec(data.output);
        const outputText = str(output?.output).trim();
        if (command && outputText) {
          parsed.messages.push({
            role: "tool",
            toolCallId: `shell-${message.id}`,
            toolName: "shell",
            toolResult: outputText,
            toolStatus: "completed",
            createdAt: createdAt ?? undefined,
          });
        }
        continue;
      }

      if (type === "compaction") {
        const status = str(data.status);
        if (status === "completed") {
          const summary = str(data.summary).trim();
          if (summary)
            parsed.messages.push({
              role: "assistant",
              text: summary,
              createdAt: createdAt ?? undefined,
            });
        }
        continue;
      }

      if (type !== "assistant") {
        parsed.skipped[`unknown-type:${type || "none"}`] =
          (parsed.skipped[`unknown-type:${type || "none"}`] ?? 0) + 1;
        continue;
      }

      const parts = (Array.isArray(data.content) ? data.content : [])
        .map(rec)
        .filter((part): part is Json => part !== null);
      const step = assistantFromParts(parts, skip, true);
      const modelSlug = str(data.modelID) || str(rec(data.model)?.id);
      if (step) {
        step.message.createdAt = createdAt ?? undefined;
        step.message.modelSlug = modelSlug || undefined;
        for (const result of step.results)
          result.createdAt = createdAt ?? undefined;
        parsed.messages.push(step.message, ...step.results);
      } else {

        const error = rec(data.error);
        const errorMessage = str(error?.message).trim();
        if (errorMessage)
          parsed.messages.push({
            role: "assistant",
            text: errorMessage,
            createdAt: createdAt ?? undefined,
          });
      }
    }

    if (parsed.messages.length > 0) {
      parsed.title = deriveSessionTitle(
        parsed.messages,
        str(row.title) || str(row.slug) || null,
      );
      sessions.push(parsed);
    } else {
      skip("no-convertible-messages");
    }
  }

  foldStoreSkips(sessions, skipCount);
  return sessions;
}

type AssistantStep = {
  message: AgentExportMessage;
  results: AgentExportMessage[];
};

function assistantFromParts(
  parts: Json[],
  skip: (reason: string, n?: number) => void,
  legacyResultBlocks = false,
): AssistantStep | null {
  const texts: string[] = [];
  const calls: AgentExportToolCall[] = [];
  const results: AgentExportMessage[] = [];

  for (const part of parts) {
    const partType = str(part.type);

    if (partType === "text") {
      const text = str(part.text).trim();
      if (text) texts.push(text);
      continue;
    }

    if (partType !== "tool") {
      skip(`part:${partType || "none"}`);
      continue;
    }

    const toolCallId = str(part.callID) || str(part.id);
    const toolName = str(part.tool) || str(part.name);
    const state = rec(part.state);
    const status = str(state?.status);
    if (!toolCallId || !toolName || status !== "completed") {
      skip("incomplete-tool-call");
      continue;
    }
    const input = rec(state?.input);
    calls.push({
      toolCallId,
      toolName,
      toolId: toolName,
      input: (input ?? {}) as Json,
    });

    const resultText = legacyResultBlocks
      ? (Array.isArray(state?.content) ? state.content : [])
          .map(rec)
          .filter(
            (item): item is Json => item !== null && str(item.type) === "text",
          )
          .map((item) => str(item.text))
          .join("\n\n")
          .trim()
      : str(state?.output).trim();
    if (resultText) {
      results.push({
        role: "tool",
        toolCallId,
        toolName,
        toolResult: resultText,
        toolStatus: "completed",
      });
    }
  }

  if (texts.length === 0 && calls.length === 0) return null;
  const message: AgentExportMessage = {
    role: "assistant",
    text: texts.join("\n\n") || undefined,
    toolCalls: calls.length > 0 ? calls : undefined,
  };
  return { message, results };
}

function foldStoreSkips(
  sessions: ParsedSession[],
  skipCount: Map<string, number>,
): void {
  if (sessions.length === 0) return;
  for (const [reason, count] of skipCount) {
    sessions[0].skipped[reason] = (sessions[0].skipped[reason] ?? 0) + count;
  }
}
