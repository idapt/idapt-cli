

export const VERB_OVERRIDES: Record<string, string> = {

  "agent edit": "agent update",

  "workspace edit": "workspace update",

  "drive write": "drive update",
  "drive edit": "drive update",
  "drive rename": "drive update",
  "drive create": "drive upload",

  "drive semantic-search": "search query",
  "drive text-search": "search query",
  "drive version": "drive content-versions",
  "drive versions": "drive content-versions",

  "hub skill-search": "hub search",
  "hub script-search": "hub search",
  "hub skill-install": "hub install",
  "hub script-install": "hub install",

  "notification read": "notification update",

  "automation edit": "automation update",
  "automation fire": "automation test-fire",

  "secret edit": "secret update",

  "utility search-llm-models": "models search",
  "utility search-image-models": "image search",
  "utility search-video-models": "video search",
  "utility search-audio-models": "audio search-models",
  "utility search-voices": "audio search-voices",
  "utility secret-list": "secret list",
  "utility secret-create": "secret create",

  "computer create": "computer ephemeral",
  "computer edit": "computer update",
  "computer terminal-list": "computer terminal",
  "computer terminal-run": "computer terminal",
  "computer terminal-capture": "computer terminal",
  "computer terminal-send-keys": "computer terminal",
  "computer terminal-kill": "computer terminal",
  "computer terminal-rename": "computer terminal",

  "computer file-read": "computer download",
  "computer file-write": "computer fs",
  "computer file-edit": "computer fs",
  "computer file-delete": "computer fs",
  "computer file-mkdir": "computer fs",
  "computer file-move": "computer fs",
  "computer file-list": "computer fs",
  "computer file-stat": "computer fs",
  "computer file-grep": "computer fs",
  "computer file-find": "computer fs",

  "computer file-upload": "computer upload",
  "computer file-download": "computer download-to-drive",
  "computer upload-dir": "computer upload",
  "computer download-dir": "computer download-to-drive",
  "computer user-list": "computer users",
  "computer user-create": "computer create-user",
  "computer user-delete": "computer delete-user",
  "computer user-edit-groups": "computer update-user",
  "computer ports-view": "computer ports",
  "computer port-label": "computer set-ports",
  "computer env-var-list": "computer user-env",
  "computer env-var-set": "computer set-user-env",
  "computer env-var-delete": "computer delete-user-env",

  "computer app list": "computer apps",
  "computer app get": "computer app-get",
  "computer app create": "computer app-create",
  "computer app run": "computer app-run",
  "computer app compose-up": "computer compose-up",
  "computer app start": "computer app-start",
  "computer app stop": "computer app-stop",
  "computer app restart": "computer app-restart",
  "computer app delete": "computer app-delete",
  "computer app logs": "computer app-logs",
  "computer app exec": "computer app-exec",
  "computer app expose": "computer app-expose",
  "computer app unexpose": "computer app-unexpose",

  "computer app status": "computer app-runtime",
  "computer app setup": "computer app-setup-runtime",
  "computer app shell": "computer app-exec",
  "computer app external": "computer app-external",
  "computer app reset": "computer app-reset",
  "computer app ports": "computer app-ports",
};

export function reconcileToV1(path: string): string {
  return VERB_OVERRIDES[path] ?? path;
}

const COLLAPSED_OPS: Record<string, string> = {
  "computer terminal-list": "list",
  "computer terminal-run": "run",
  "computer terminal-capture": "capture",
  "computer terminal-send-keys": "send",
  "computer terminal-kill": "kill",
  "computer terminal-rename": "rename",
  "computer file-write": "write",
  "computer file-edit": "edit",
  "computer file-delete": "delete",
  "computer file-mkdir": "mkdir",
  "computer file-move": "move",
  "computer file-list": "list",
  "computer file-stat": "stat",
  "computer file-grep": "grep",
  "computer file-find": "find",
};

export const PATH_PARAM_ALIASES: Record<string, string[]> = {
  id: [
    "id",
    "resource_id",
    "file_id",
    "computer_id",
    "agent_id",
    "chat_id",
    "workspace_id",
    "trigger_id",
    "hook_id",
    "notification_id",

    "name",
    "path",
  ],
  app_id: ["app_id", "app_name", "app"],
  username: ["username", "user"],
  secret_id: ["secret_id", "secret"],
};

const ARG_RENAMES: Record<string, Record<string, string>> = {

  "search query": { query: "q" },

  "inference image": {
    path: "output_path",
    image_references: "reference_image_paths",
  },
  "inference video": {
    path: "output_path",
    image_references: "reference_image_paths",
    video_reference: "video_reference_path",
  },

  "inference speech": {
    path: "output_path",
  },

  "drive update": {
    new_name: "name",
  },

  "computer upload": {
    file_path: "path",
    source_path: "drive_source",
  },
  "computer download-to-drive": {
    file_path: "path",
    destination_path: "drive_destination",
  },
};

const RESOLVED_FIELD_TO_V1: Record<string, string> = {
  resolved_file_id: "id",
  resolved_folder_id: "id",
  resolved_computer: "id",
  resolved_agent: "id",
  resolved_script: "id",
  resolved_source_id: "source_id",
  resolved_secret_ids: "secret_ids",
  resolved_parent_id: "parent_id",

  resolved_new_parent_id: "parent_id",
  resolved_workspace_id: "workspace_id",
};

function base64ToUtf8(b64: string): string {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(b64, "base64").toString("utf-8");
  }
  const bin = atob(b64);
  const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function toSnake(key: string): string {
  return key
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/-/g, "_")
    .toLowerCase();
}

export function mapArgsToV1(
  path: string,
  pathParams: readonly string[],
  args: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(args)) out[toSnake(k)] = v;

  const reconciled = reconcileToV1(path);

  if (reconciled === "drive update" || reconciled === "drive upload") {
    if (typeof out.content_b64 === "string" && out.content === undefined) {
      out.content = base64ToUtf8(out.content_b64);
    }
    delete out.content_b64;
  }

  const op = COLLAPSED_OPS[path];
  if (op && out.op === undefined) out.op = op;

  if (
    path === "drive create" &&
    out.file === undefined &&
    typeof out.content === "string"
  ) {
    const target = String(out.path ?? out.name ?? "untitled.txt");
    const filename = target.split("/").pop() || target;
    out.file = new File([out.content], filename, { type: "text/plain" });
    delete out.content;
    delete out.path;
  }

  if (
    path === "drive create-folder" &&
    out.name === undefined &&
    typeof out.path === "string"
  ) {
    const target = out.path;
    out.name = target.split("/").pop() || target;
    delete out.path;
  }

  const resolvedParent = out.resolved_parent_id;
  for (const [from, to] of Object.entries(RESOLVED_FIELD_TO_V1)) {
    const val = out[from];
    if (val === undefined) continue;
    delete out[from];

    if (to === "workspace_id" && resolvedParent != null) continue;
    if (val !== null && out[to] === undefined) out[to] = val;
  }
  for (const key of Object.keys(out)) {
    if (key.startsWith("resolved_")) delete out[key];
  }

  for (const param of pathParams) {
    if (out[param] !== undefined) continue;
    let aliases = PATH_PARAM_ALIASES[param] ?? [param];

    if (param === "id" && !path.startsWith("workspace ")) {
      aliases = aliases.filter((a) => a !== "workspace_id");
    }
    for (const alias of aliases) {
      if (out[alias] !== undefined) {
        out[param] = out[alias];
        break;
      }
    }
  }

  const renames = ARG_RENAMES[reconciled];
  if (renames) {
    for (const [from, to] of Object.entries(renames)) {
      if (out[from] !== undefined && out[to] === undefined) {
        out[to] = out[from];
        delete out[from];
      }
    }
  }
  return out;
}
