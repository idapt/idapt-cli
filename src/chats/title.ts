

const SYNTHETIC_MARKERS = [
  "<local-command-caveat>",
  "<local-command-stdout>",
  "<local-command-stderr>",
  "<command-name>",
  "<command-message>",
  "<command-args>",
  "<command-contents>",
  "<task-notification>",
  "<system-reminder>",
  "<INSTRUCTIONS>",
  "<user_instructions>",
  "<USER_INSTRUCTIONS>",
  "<environment_context>",
  "<ENVIRONMENT_CONTEXT>",
  "# AGENTS.md instructions",
  "Caveat: The messages below",
] as const;

export function isSyntheticMessage(text: string): boolean {
  const trimmed = text.trimStart().toLowerCase();
  return SYNTHETIC_MARKERS.some((marker) => {
    const lower = marker.toLowerCase();
    return trimmed.startsWith(lower) || trimmed === lower;
  });
}

export function oneLineTitle(text: string, max = 120): string {
  const flat = text.replace(/\s+/g, " ").trim();
  if (flat.length <= max) return flat || "Untitled session";
  return `${flat.slice(0, max - 1)}…`;
}

export type TitleCandidate = {
  role: string;
  text?: string;

  toolResult?: string;
};

export function deriveSessionTitle(
  messages: readonly TitleCandidate[],
  storedTitle?: string | null,
  fallback = "Untitled session",
): string {
  if (storedTitle && !isSyntheticMessage(storedTitle))
    return oneLineTitle(storedTitle);
  for (const message of messages) {
    if (message.role !== "user" || !message.text) continue;
    if (isSyntheticMessage(message.text)) continue;
    return oneLineTitle(message.text);
  }
  return oneLineTitle(fallback);
}
