

import { createReadStream } from "node:fs";
import path from "node:path";
import { createInterface } from "node:readline";
import type {
  AgentExportMessage,
  AgentExportToolCall,
} from "@shared/chat/agent-export";
import type { AgentStoreProject } from "../detect";
import { deriveSessionTitle } from "../title";

export type ParsedSession = {
  sourceId: string;
  title: string;
  project?: string;
  createdAt: Date;
  updatedAt: Date;
  messages: AgentExportMessage[];

  skipped: Record<string, number>;
};

type Json = Record<string, unknown>;

function rec(value: unknown): Json | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Json)
    : null;
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function parseTimestamp(value: unknown): Date | null {
  if (typeof value === "string" && value) {
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) return new Date(parsed);
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return new Date(value < 1e12 ? value * 1000 : value);
  }
  return null;
}

function toolResultText(value: unknown): string {
  if (typeof value === "string") return value;
  const blocks = Array.isArray(value) ? value : [];
  const parts: string[] = [];
  for (const block of blocks) {
    const record = rec(block);
    if (!record) continue;
    if (str(record.type) === "text") parts.push(str(record.text));
  }
  return parts.join("\n\n");
}

function decodeRecord(line: string): {
  sessionId: string;
  record: Json;
  kind: "conversation" | "skipped";
  skipReason?: string;
} | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(line);
  } catch {
    return {
      sessionId: "",
      record: {},
      kind: "skipped",
      skipReason: "malformed-json",
    };
  }
  const record = rec(parsed);
  if (!record) {
    return {
      sessionId: "",
      record: {},
      kind: "skipped",
      skipReason: "malformed-json",
    };
  }
  const sessionId = str(record.sessionId);
  const type = str(record.type);
  if (str(record.isSidechain) === "true" || record.isSidechain === true) {
    return { sessionId, record, kind: "skipped", skipReason: "sidechain" };
  }
  if (type === "summary" || type === "system") {
    return { sessionId, record, kind: "skipped", skipReason: type };
  }
  if (type !== "user" && type !== "assistant") {
    return {
      sessionId,
      record,
      kind: "skipped",
      skipReason: `unknown-type:${type || "none"}`,
    };
  }
  return { sessionId, record, kind: "conversation" };
}

export async function readClaudeCodeProject(
  project: AgentStoreProject,
): Promise<ParsedSession[]> {
  const sessions = new Map<string, ParsedSession>();

  const callNames = new Map<string, Map<string, string>>();

  for (const file of project.entries) {
    const stream = createReadStream(file, { encoding: "utf8" });
    const lines = createInterface({ input: stream, crlfDelay: Infinity });

    let fileSessionId = "";

    let pendingSummaryTitle: string | null = null;
    let pendingSummarySessionId: string | null = null;
    for await (const line of lines) {
      if (!line.trim()) continue;
      const decoded = decodeRecord(line);
      if (!decoded) continue;
      if (decoded.sessionId) fileSessionId = decoded.sessionId;
      const sessionId = decoded.sessionId || fileSessionId;
      if (decoded.kind === "skipped") {
        if (sessionId) {
          const target = sessions.get(sessionId);
          if (target) {
            target.skipped[decoded.skipReason ?? "other"] =
              (target.skipped[decoded.skipReason ?? "other"] ?? 0) + 1;
          }
        }

        if (decoded.skipReason === "summary") {
          const text = str(decoded.record.summary).trim();
          if (text) {
            const target = sessionId ? sessions.get(sessionId) : undefined;
            if (target) {
              if (target.title === "Untitled session") target.title = text;
            } else {
              pendingSummaryTitle = text.slice(0, 200);
              pendingSummarySessionId = decoded.sessionId || null;
            }
          }
        }
        continue;
      }
      if (!sessionId) continue;
      let target = sessions.get(sessionId);
      if (!target) {
        target = {
          sourceId: sessionId,
          title: "Untitled session",
          createdAt: new Date(0),
          updatedAt: new Date(0),
          messages: [],
          skipped: {},
        };

        if (
          pendingSummaryTitle &&
          (pendingSummarySessionId === null ||
            pendingSummarySessionId === sessionId)
        ) {
          target.title = pendingSummaryTitle;
          pendingSummaryTitle = null;
        }
        sessions.set(sessionId, target);
      }
      const timestamp = parseTimestamp(decoded.record.timestamp);
      const message = rec(decoded.record.message);
      const _role = str(message?.role);
      const cwd = str(decoded.record.cwd);
      if (cwd) target.project = cwd;
      if (timestamp) {
        if (target.createdAt.getTime() === 0) target.createdAt = timestamp;
        if (timestamp > target.updatedAt) target.updatedAt = timestamp;
      }

      if (!message) continue;
      const content = message.content;

      if (decoded.record.type === "assistant") {
        const calls: AgentExportToolCall[] = [];
        const texts: string[] = [];
        for (const block of Array.isArray(content) ? content : []) {
          const blockRecord = rec(block);
          if (!blockRecord) continue;
          const blockType = str(blockRecord.type);
          if (blockType === "text") {
            const text = str(blockRecord.text).trim();
            if (text) texts.push(text);
          } else if (blockType === "tool_use") {
            const toolCallId = str(blockRecord.id);
            const toolName = str(blockRecord.name);
            if (!toolCallId || !toolName) continue;
            calls.push({
              toolCallId,
              toolName,
              toolId: toolName,
              input: rec(blockRecord.input) ?? {},
            });
            let names = callNames.get(sessionId);
            if (!names) {
              names = new Map();
              callNames.set(sessionId, names);
            }
            names.set(toolCallId, toolName);
          }

        }
        if (texts.length === 0 && calls.length === 0) continue;
        const model = str(message.model);
        target.messages.push({
          role: "assistant",
          text: texts.join("\n\n") || undefined,
          modelSlug: model || undefined,
          toolCalls: calls.length > 0 ? calls : undefined,
          createdAt: timestamp ?? undefined,
        });
        continue;
      }

      if (record2IsCompact(decoded.record)) {
        const text =
          typeof content === "string"
            ? content.trim()
            : (Array.isArray(content) ? content : [])
                .map(rec)
                .filter(
                  (block): block is Json =>
                    block !== null && str(block.type) === "text",
                )
                .map((block) => str(block.text).trim())
                .filter(Boolean)
                .join("\n\n");
        if (text)
          target.messages.push({
            role: "compaction",
            text,
            createdAt: timestamp ?? undefined,
          });
        continue;
      }
      if (typeof content === "string") {
        const text = content.trim();
        if (text)
          target.messages.push({
            role: "user",
            text,
            createdAt: timestamp ?? undefined,
          });
        continue;
      }
      for (const block of Array.isArray(content) ? content : []) {
        const blockRecord = rec(block);
        if (!blockRecord) continue;
        if (str(blockRecord.type) === "tool_result") {
          const toolCallId = str(blockRecord.tool_use_id);
          if (!toolCallId) continue;
          const text = toolResultText(blockRecord.content);
          if (!text) continue;
          target.messages.push({
            role: "tool",
            toolCallId,
            toolName: callNames.get(sessionId)?.get(toolCallId) ?? undefined,
            toolResult: text,
            toolStatus: "completed",
            createdAt: timestamp ?? undefined,
          });
          continue;
        }
        if (str(blockRecord.type) === "text") {
          const text = str(blockRecord.text).trim();
          if (text)
            target.messages.push({
              role: "user",
              text,
              createdAt: timestamp ?? undefined,
            });
        }
      }
    }
  }

  const parsed = [...sessions.values()];
  for (const session of parsed) {
    session.title = deriveSessionTitle(
      session.messages,
      session.title === "Untitled session" ? null : session.title,
    );
  }
  return parsed.filter((session) => session.messages.length > 0);
}

function record2IsCompact(record: Json): boolean {
  return record.isCompactSummary === true;
}

export function claudeCodeProjectLabel(project: AgentStoreProject): string {
  return project.entries[0]
    ? path.basename(path.dirname(project.entries[0]))
    : project.name;
}
