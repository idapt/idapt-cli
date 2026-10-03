

import { createRequire } from "node:module";
import type {
  AgentExportMessage,
  AgentExportToolCall,
} from "@shared/chat/agent-export";
import type { ParsedSession } from "./claude-code";

const { DatabaseSync } = createRequire(import.meta.url)(
  "node:sqlite",
) as typeof import("node:sqlite");

type Json = Record<string, unknown>;

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

export async function readOpenCodeDatabase(
  dbPath: string,
): Promise<ParsedSession[]> {
  const db = new DatabaseSync(dbPath, { readOnly: true });

  const sessions: ParsedSession[] = [];
  const skipCount = new Map<string, number>();
  const skip = (reason: string, n = 1): void => {
    skipCount.set(reason, (skipCount.get(reason) ?? 0) + n);
  };

  try {
    const sessionRows = db
      .prepare(
        "SELECT id, directory, title, slug, parent_id, time_created, time_updated FROM session_v2 ORDER BY time_created",
      )
      .all() as Array<{
      id: string;
      directory: string | null;
      title: string | null;
      slug: string | null;
      parent_id: string | null;
      time_created: number | null;
      time_updated: number | null;
    }>;

    const messageStmt = db.prepare(
      "SELECT id, type, seq, time_created, time_updated, data FROM session_message WHERE session_id = ? ORDER BY seq",
    );

    for (const row of sessionRows) {
      if (row.parent_id) {
        skip("subagent-session");
        continue;
      }
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

        const texts: string[] = [];
        const calls: AgentExportToolCall[] = [];
        const stepRows: AgentExportMessage[] = [];
        for (const part of Array.isArray(data.content) ? data.content : []) {
          const partRecord = rec(part);
          if (!partRecord) continue;
          const partType = str(partRecord.type);

          if (partType === "text") {
            const text = str(partRecord.text).trim();
            if (text) texts.push(text);
            continue;
          }

          if (partType !== "tool") continue;
          const toolCallId = str(partRecord.id);
          const toolName = str(partRecord.name);
          const state = rec(partRecord.state);
          const status = str(state?.status);
          if (!toolCallId || !toolName || status !== "completed") {
            parsed.skipped["incomplete-tool-call"] =
              (parsed.skipped["incomplete-tool-call"] ?? 0) + 1;
            continue;
          }
          const input = rec(state?.input);
          calls.push({
            toolCallId,
            toolName,
            toolId: toolName,
            input: (input ?? {}) as Json,
          });
          const resultParts: string[] = [];
          for (const item of Array.isArray(state?.content)
            ? state.content
            : []) {
            const itemRecord = rec(item);
            if (itemRecord && str(itemRecord.type) === "text") {
              resultParts.push(str(itemRecord.text));
            }
          }
          const resultText = resultParts.join("\n\n").trim();
          if (resultText) {

            stepRows.push({
              role: "tool",
              toolCallId,
              toolName,
              toolResult: resultText,
              toolStatus: "completed",
            });
          }
        }

        if (texts.length === 0 && calls.length === 0) {
          const error = rec(data.error);
          const errorMessage = str(error?.message).trim();
          if (errorMessage) {
            parsed.messages.push({
              role: "assistant",
              text: errorMessage,
              createdAt: createdAt ?? undefined,
            });
          }
          continue;
        }

        const model = rec(data.model);
        const step: AgentExportMessage = {
          role: "assistant",
          text: texts.join("\n\n") || undefined,
          modelSlug: str(model?.id) || undefined,
          toolCalls: calls.length > 0 ? calls : undefined,
        };
        step.createdAt = createdAt ?? undefined;
        for (const row of stepRows) row.createdAt = createdAt ?? undefined;
        parsed.messages.push(step);
        parsed.messages.push(...stepRows);
      }

      if (parsed.messages.length > 0) sessions.push(parsed);
      else skip("no-convertible-messages");
    }
  } finally {
    db.close();
  }

  const sessionLevel: Record<string, number> = {};
  for (const [reason, count] of skipCount) sessionLevel[reason] = count;
  if (sessions.length > 0) {
    for (const [reason, count] of Object.entries(sessionLevel)) {
      sessions[0].skipped[reason] = (sessions[0].skipped[reason] ?? 0) + count;
    }
  }
  return sessions;
}
