

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

export type DetectedSkill = {

  name: string;
  description?: string;

  body: string;

  folder: string;
  path: string;
};

function parseFrontmatterField(
  block: string,
  field: string,
): string | undefined {
  const match = block.match(new RegExp(`^${field}:\\s*(.+)$`, "m"));
  return match?.[1]?.trim() || undefined;
}

export function parseSkillMd(
  source: string,
): Pick<DetectedSkill, "name" | "description" | "body"> | null {
  const normalized = source.replace(/\r\n/g, "\n");
  const match = normalized.match(/^---\n([\s\S]*?)\n---\n?/);
  const body = match
    ? normalized.slice(match[0].length).trim()
    : normalized.trim();
  const meta = match ? match[1] : "";
  const name = parseFrontmatterField(meta, "name");
  const description = parseFrontmatterField(meta, "description");
  if (!body) return null;
  return { name: name ?? "", description, body };
}

function skillsInDir(root: string): DetectedSkill[] {
  const found: DetectedSkill[] = [];
  let entries: string[] = [];
  try {
    entries = readdirSync(root);
  } catch {
    return found;
  }
  for (const entry of entries) {
    const dir = path.join(root, entry);
    if (!statSync(dir).isDirectory()) continue;

    let candidates = [dir];
    try {
      candidates = candidates.concat(
        readdirSync(dir)
          .filter((sub) => statSync(path.join(dir, sub)).isDirectory())
          .map((sub) => path.join(dir, sub)),
      );
    } catch {

    }
    for (const candidate of candidates) {
      const skillMd = path.join(candidate, "SKILL.md");
      if (!existsSync(skillMd)) continue;
      let raw: string;
      try {
        raw = readFileSync(skillMd, "utf8");
      } catch {
        continue;
      }
      const parsed = parseSkillMd(raw);
      if (!parsed) continue;
      found.push({
        ...parsed,
        name: parsed.name || entry,
        folder: entry,
        path: skillMd,
      });
    }
  }
  return found;
}

export function detectSkills(opts: {
  home: string;
  cwd: string;
  env: Record<string, string | undefined>;
}): DetectedSkill[] {
  const configDir =
    opts.env.CLAUDE_CONFIG_DIR || path.join(opts.home, ".claude");
  const roots = [
    path.join(configDir, "skills"),
    path.join(opts.cwd, ".claude", "skills"),
  ];
  const byFolder = new Map<string, DetectedSkill>();
  for (const root of roots) {
    for (const skill of skillsInDir(root)) {
      const existing = byFolder.get(skill.folder);
      if (existing && existing.body === skill.body) continue;
      byFolder.set(skill.folder, skill);
    }
  }
  return [...byFolder.values()].sort((a, b) => a.name.localeCompare(b.name));
}
