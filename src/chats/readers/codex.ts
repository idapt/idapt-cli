

import { createReadStream } from "node:fs";
import path from "node:path";
import { createInterface } from "node:readline";
import type { AgentExportToolCall } from "@shared/chat/agent-export";
import type { ParsedSession } from "./claude-code";

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

function concatParts(content: unknown, partType: string): string {
  const parts: string[] = [];
  for (const item of Array.isArray(content) ? content : []) {
    const itemRecord = rec(item);
    if (!itemRecord) continue;
    if (str(itemRecord.type) === partType) parts.push(str(itemRecord.text));
  }
  return parts.join("\n\n").trim();
}

export async function readCodexRollout(
  file: string,
): Promise<ParsedSession | null> {
  const stream = createReadStream(file, { encoding: "utf8" });
  const lines = createInterface({ input: stream, crlfDelay: Infinity });

  const session: ParsedSession = {
    sourceId: "",
    title: "Untitled session",
    createdAt: new Date(0),
    updatedAt: new Date(0),
    messages: [],
    skipped: {},
  };

  const callNames = new Map<string, string>();
  let firstUserText: string | null = null;
  let sawCanonical = false;

  const skip = (reason: string): void => {
    session.skipped[reason] = (session.skipped[reason] ?? 0) + 1;
  };

  for await (const line of lines) {
    if (!line.trim()) continue;
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch {
      skip("malformed-json");
      continue;
    }
    const record = rec(parsed);
    if (!record) {
      skip("malformed-json");
      continue;
    }
    const timestamp = parseTimestamp(record.timestamp);
    const payload = rec(record.payload) ?? {};
    const type = str(record.type);

    if (type === "session_meta") {
      const id = str(payload.id);
      if (id) session.sourceId = id;
      const cwd = str(payload.cwd);
      if (cwd) session.project = cwd;
      if (timestamp && session.createdAt.getTime() === 0)
        session.createdAt = timestamp;
      continue;
    }

    if (type !== "response_item") {
      if (type === "event_msg" && !sawCanonical) {

        if (str(payload.type) === "user_message") {
          const message = str(payload.message).trim();
          if (message && !firstUserText) firstUserText = message;
        }
      }
      skip(`unknown-type:${type || "none"}`);
      continue;
    }
    sawCanonical = true;

    const itemType = str(payload.type);

    if (itemType === "message") {
      const role = str(payload.role);
      if (role === "user") {
        const text = concatParts(payload.content, "input_text");
        if (text) {
          session.messages.push({
            role: "user",
            text,
            createdAt: timestamp ?? undefined,
          });
          if (!firstUserText) firstUserText = text;
        }
        continue;
      }
      if (role === "assistant") {
        const text = concatParts(payload.content, "output_text");
        const model = rec(payload.model);
        if (text) {
          session.messages.push({
            role: "assistant",
            text,
            modelSlug: str(model?.id) || undefined,
            createdAt: timestamp ?? undefined,
          });
        }
        continue;
      }
      skip(`unknown-role:${role || "none"}`);
      continue;
    }

    if (itemType === "function_call") {
      const callId = str(payload.call_id);
      const name = str(payload.name);
      if (!callId || !name) {
        skip("unpaired-tool-call");
        continue;
      }
      callNames.set(callId, name);
      let input: Json = {};
      try {
        const args = JSON.parse(str(payload.arguments) || "{}");
        if (rec(args)) input = args as Json;
      } catch {

      }
      const calls: AgentExportToolCall[] = [
        { toolCallId: callId, toolName: name, toolId: name, input },
      ];

      const last = session.messages[session.messages.length - 1];
      if (last && last.role === "assistant" && last.toolCalls) {
        last.toolCalls.push(...calls);
      } else if (
        last &&
        last.role === "assistant" &&
        !last.text &&
        !last.toolCalls
      ) {
        last.toolCalls = calls;
      } else {
        session.messages.push({
          role: "assistant",
          toolCalls: calls,
          createdAt: timestamp ?? undefined,
        });
      }
      continue;
    }

    if (itemType === "function_call_output") {
      const callId = str(payload.call_id);
      const output = str(payload.output).trim();
      if (!callId || !output) {
        skip("empty-tool-result");
        continue;
      }
      session.messages.push({
        role: "tool",
        toolCallId: callId,
        toolName: callNames.get(callId) ?? undefined,
        toolResult: output,
        toolStatus: "completed",
        createdAt: timestamp ?? undefined,
      });
      continue;
    }

    skip(`unknown-item:${itemType || "none"}`);
  }

  if (!session.sourceId) {

    const match = /rollout-[\dT-]+-([0-9a-f-]{36})\.jsonl$/.exec(file);
    if (match) session.sourceId = match[1];
  }
  if (timestampFallback(file) > session.updatedAt.getTime())
    session.updatedAt = new Date(timestampFallback(file));
  if (session.createdAt.getTime() === 0)
    session.createdAt = new Date(timestampFallback(file));
  session.title = (firstUserText ?? session.title).slice(0, 200);
  if (session.messages.length === 0) return null;
  return session;
}

function timestampFallback(file: string): number {
  const match =
    /rollout-(\d{4})-(\d{2})-(\d{2})[T-](\d{2})-(\d{2})-(\d{2})/.exec(
      path.basename(file),
    );
  if (!match) return 0;
  const [, y, mo, d, h, mi, s] = match;
  return Date.UTC(+y, +mo - 1, +d, +h, +mi, +s);
}
