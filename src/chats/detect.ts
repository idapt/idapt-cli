

import { existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import type { AgentExportSource } from "@shared/chat/agent-export";
import { AGENT_EXPORT_SOURCE_LABELS } from "@shared/chat/agent-export";

export type AgentStoreProject = {

  name: string;

  path: string;

  entries: string[];
};

export type DetectedAgentStore = {
  source: AgentExportSource;
  label: string;

  root: string | null;
  installed: boolean;

  reason?: string;
  projects: AgentStoreProject[];
};

export type ScanPaths = {
  home: string;
  env: Record<string, string | undefined>;
};

function jsonlFilesIn(dir: string): string[] {
  try {
    return readdirSync(dir)
      .filter((f) => f.endsWith(".jsonl"))
      .map((f) => path.join(dir, f))
      .filter((f) => statSync(f).isFile());
  } catch {
    return [];
  }
}

function scanClaudeCode(scan: ScanPaths): DetectedAgentStore {
  const configDir =
    scan.env.CLAUDE_CONFIG_DIR || path.join(scan.home, ".claude");
  const projectsDir = path.join(configDir, "projects");
  if (!existsSync(projectsDir)) {
    return {
      source: "claude-code",
      label: AGENT_EXPORT_SOURCE_LABELS["claude-code"],
      root: configDir,
      installed: false,
      reason: `no session stores found (${projectsDir} missing)`,
      projects: [],
    };
  }
  const projects: AgentStoreProject[] = [];
  for (const entry of readdirSync(projectsDir)) {
    const dir = path.join(projectsDir, entry);
    if (!statSync(dir).isDirectory()) continue;
    const files = jsonlFilesIn(dir);
    if (files.length === 0) continue;
    projects.push({

      name: entry,
      path: entry,
      entries: files,
    });
  }
  return {
    source: "claude-code",
    label: AGENT_EXPORT_SOURCE_LABELS["claude-code"],
    root: projectsDir,
    installed: true,
    projects,
  };
}

function scanCodex(scan: ScanPaths): DetectedAgentStore {
  const codexHome = scan.env.CODEX_HOME || path.join(scan.home, ".codex");
  const sessionsDir = path.join(codexHome, "sessions");
  if (!existsSync(sessionsDir)) {
    return {
      source: "codex",
      label: AGENT_EXPORT_SOURCE_LABELS.codex,
      root: codexHome,
      installed: false,
      reason: `no session stores found (${sessionsDir} missing)`,
      projects: [],
    };
  }

  const rollouts: string[] = [];
  const walk = (dir: string, depth: number): void => {
    if (depth > 4) return;
    for (const entry of readdirSync(dir)) {
      const full = path.join(dir, entry);
      const stat = statSync(full);
      if (stat.isDirectory()) walk(full, depth + 1);
      else if (entry.endsWith(".jsonl") && stat.isFile()) rollouts.push(full);
    }
  };
  walk(sessionsDir, 0);
  return {
    source: "codex",
    label: AGENT_EXPORT_SOURCE_LABELS.codex,
    root: sessionsDir,
    installed: true,
    projects: rollouts.length
      ? [{ name: "all", path: "all", entries: rollouts }]
      : [],
  };
}

function scanOpenCode(scan: ScanPaths): DetectedAgentStore {
  const dataDir =
    scan.env.XDG_DATA_HOME || path.join(scan.home, ".local", "share");
  const storeRoot = path.join(dataDir, "opencode");
  if (!existsSync(storeRoot)) {
    return {
      source: "opencode",
      label: AGENT_EXPORT_SOURCE_LABELS.opencode,
      root: storeRoot,
      installed: false,
      reason: `no OpenCode data directory (${storeRoot} missing)`,
      projects: [],
    };
  }
  let dbPath: string | null = null;
  for (const candidate of ["opencode.db", "opencode-latest.db"]) {
    const full = path.join(storeRoot, candidate);
    if (existsSync(full)) {
      dbPath = full;
      break;
    }
  }
  if (!dbPath) {
    for (const entry of readdirSync(storeRoot).filter((f) =>
      /^opencode-(.+)\.db$/.test(f),
    )) {
      dbPath = path.join(storeRoot, entry);
      break;
    }
  }
  if (!dbPath) {
    return {
      source: "opencode",
      label: AGENT_EXPORT_SOURCE_LABELS.opencode,
      root: storeRoot,
      installed: false,
      reason: `no session database found in ${storeRoot}`,
      projects: [],
    };
  }
  return {
    source: "opencode",
    label: AGENT_EXPORT_SOURCE_LABELS.opencode,
    root: dbPath,
    installed: true,
    projects: [],
  };
}

export function detectAgentStores(scan: ScanPaths): DetectedAgentStore[] {
  return [scanClaudeCode(scan), scanCodex(scan), scanOpenCode(scan)];
}

export function detectedStoreFor(
  stores: DetectedAgentStore[],
  source: AgentExportSource,
): DetectedAgentStore | undefined {
  return stores.find((store) => store.source === source);
}
