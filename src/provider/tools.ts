

import { homedir } from "node:os";
import { join } from "node:path";
import { mergeJsonConfig, readJsonPath } from "./json-config";
import {
  readTomlKey,
  removeTomlSection,
  removeTomlTopLevelKey,
  upsertTomlSection,
  upsertTomlTopLevel,
} from "./toml";

export interface ProviderContext {

  baseUrl: string;

  providerId: string;

  envKey: string;

  model?: string;
}

export type ToolFormat = "toml" | "json";

export interface EditResult {
  text: string;
  changed: boolean;
}

export interface ToolDescriptor {
  readonly id: string;
  readonly label: string;
  readonly format: ToolFormat;

  configPath(env: Record<string, string | undefined>): string;
  apply(text: string, ctx: ProviderContext): EditResult;
  remove(text: string, ctx: ProviderContext): EditResult;

  verify(text: string, ctx: ProviderContext): string[];

  nextSteps(ctx: ProviderContext): string[];
}

function configHome(env: Record<string, string | undefined>): string {
  if (process.platform === "win32") {
    return env.APPDATA ?? join(homedir(), "AppData", "Roaming");
  }
  if (process.platform === "darwin") {
    return join(homedir(), "Library", "Application Support");
  }
  return env.XDG_CONFIG_HOME ?? join(homedir(), ".config");
}

const CODEX: ToolDescriptor = {
  id: "codex",
  label: "Codex CLI",
  format: "toml",
  configPath: (env) =>
    join(env.CODEX_HOME ?? join(homedir(), ".codex"), "config.toml"),
  apply(text, ctx) {
    let out = text;
    let changed = false;
    const top = upsertTomlTopLevel(out, { model_provider: ctx.providerId });
    out = top.text;
    changed = changed || top.changed;
    if (ctx.model) {
      const withModel = upsertTomlTopLevel(out, { model: ctx.model });
      out = withModel.text;
      changed = changed || withModel.changed;
    }
    const section = upsertTomlSection(
      out,
      ["model_providers", ctx.providerId],
      {
        name: "idapt",
        base_url: ctx.baseUrl,
        wire_api: "responses",
        env_key: ctx.envKey,
      },
    );
    out = section.text;
    changed = changed || section.changed;
    return { text: out, changed };
  },
  remove(text, ctx) {
    let out = text;
    let changed = false;
    const section = removeTomlSection(out, ["model_providers", ctx.providerId]);
    out = section.text;
    changed = changed || section.changed;
    if (readTomlKey(out, null, "model_provider") === ctx.providerId) {
      const top = removeTomlTopLevelKey(out, "model_provider");
      out = top.text;
      changed = changed || top.changed;
    }
    return { text: out, changed };
  },
  verify(text, ctx) {
    const issues: string[] = [];
    const base = readTomlKey(
      text,
      ["model_providers", ctx.providerId],
      "base_url",
    );
    if (base !== ctx.baseUrl) {
      issues.push(
        `model_providers.${ctx.providerId}.base_url is ${base ?? "(missing)"}, expected ${ctx.baseUrl}`,
      );
    }
    const wire = readTomlKey(
      text,
      ["model_providers", ctx.providerId],
      "wire_api",
    );
    if (wire !== "responses") {
      issues.push(
        `model_providers.${ctx.providerId}.wire_api is ${wire ?? "(missing)"}, expected "responses"`,
      );
    }
    if (readTomlKey(text, null, "model_provider") !== ctx.providerId) {
      issues.push(`top-level model_provider is not "${ctx.providerId}"`);
    }
    return issues;
  },
  nextSteps(ctx) {
    return [
      `export ${ctx.envKey}="$(idapt auth token)"   # or paste your idapt API key`,
      "codex",
    ];
  },
};

const OPENCODE: ToolDescriptor = {
  id: "opencode",
  label: "opencode",
  format: "json",
  configPath: (env) => join(configHome(env), "opencode", "opencode.json"),
  apply(text, ctx) {
    return mergeJsonConfig(text, {
      provider: {
        [ctx.providerId]: {
          npm: "@ai-sdk/openai-compatible",
          name: "idapt",
          options: {
            baseURL: ctx.baseUrl,
            apiKey: `{env:${ctx.envKey}}`,
          },
        },
      },
    });
  },
  remove(text, ctx) {
    if (text.trim() === "") return { text, changed: false };
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(text) as Record<string, unknown>;
    } catch {
      return { text, changed: false };
    }
    const provider = parsed.provider as Record<string, unknown> | undefined;
    if (!provider || !(ctx.providerId in provider)) {
      return { text, changed: false };
    }
    delete provider[ctx.providerId];
    if (Object.keys(provider).length === 0) delete parsed.provider;
    return { text: `${JSON.stringify(parsed, null, 2)}\n`, changed: true };
  },
  verify(text, ctx) {
    const base = readJsonPath(text, [
      "provider",
      ctx.providerId,
      "options",
      "baseURL",
    ]);
    if (base !== ctx.baseUrl) {
      return [
        `provider.${ctx.providerId}.options.baseURL is ${String(base ?? "(missing)")}, expected ${ctx.baseUrl}`,
      ];
    }
    return [];
  },
  nextSteps(ctx) {
    return [`export ${ctx.envKey}="$(idapt auth token)"`, "opencode"];
  },
};

export const TOOLS: readonly ToolDescriptor[] = [CODEX, OPENCODE];

export function findTool(id: string): ToolDescriptor | undefined {
  return TOOLS.find((tool) => tool.id === id);
}

export function openAIBaseUrl(appBaseUrl: string): string {
  const trimmed = appBaseUrl.replace(/\/+$/, "");
  return `${trimmed}/api/openai/v1`;
}

export const DEFAULT_PROVIDER_ID = "idapt";
export const DEFAULT_ENV_KEY = "IDAPT_API_KEY";
