

import type { RenderMode } from "./format";

export const OUTPUT_MODES: readonly RenderMode[] = [
  "table",
  "json",
  "jsonl",
  "quiet",
];

export interface GlobalFlags {

  output?: RenderMode;

  workspace?: string;

  to?: string;

  agent?: string;

  context?: string;
  apiKey?: string;
  apiUrl?: string;

  timeoutSeconds?: number;
  verbose?: boolean;
  noColor?: boolean;

  yes?: boolean;

  all?: boolean;

  columns?: string[];

  filter?: string[];

  sort?: string;

  background?: boolean;

  file?: string;
  help?: boolean;
  instructions?: boolean;
  version?: boolean;
}

export interface ParsedArgv {
  readonly globals: GlobalFlags;

  readonly rest: string[];

  readonly restFlagNames: string[];

  readonly errors: string[];
}

type GlobalSpec =
  | {
      kind: "boolean";
      names: readonly string[];
      apply: (f: GlobalFlags) => void;
    }
  | {
      kind: "value";
      names: readonly string[];

      valueName: string;

      allowsLeadingDash?: boolean;
      apply: (f: GlobalFlags, value: string) => string | undefined;
    };

const GLOBALS: readonly GlobalSpec[] = [
  {
    kind: "value",
    names: ["--output", "-o"],
    valueName: "mode",
    apply: (f, v) => {
      if (!(OUTPUT_MODES as readonly string[]).includes(v)) {
        return `invalid --output mode "${v}". Expected one of: ${OUTPUT_MODES.join(", ")}.`;
      }
      f.output = v as RenderMode;
      return undefined;
    },
  },
  {
    kind: "boolean",
    names: ["--quiet", "-q"],
    apply: (f) => {
      f.output = "quiet";
    },
  },
  {
    kind: "value",
    names: ["--workspace", "-w"],
    valueName: "workspace",
    apply: (f, v) => {
      f.workspace = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--to"],
    valueName: "workspace",
    apply: (f, v) => {
      f.to = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--agent"],
    valueName: "agent",
    apply: (f, v) => {
      f.agent = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--context"],
    valueName: "name",
    apply: (f, v) => {
      f.context = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--api-key"],
    valueName: "key",
    apply: (f, v) => {
      f.apiKey = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--api-url"],
    valueName: "url",
    apply: (f, v) => {
      f.apiUrl = v;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--timeout"],
    valueName: "seconds",
    apply: (f, v) => {
      const seconds = Number(v.replace(/s$/, ""));
      if (!Number.isFinite(seconds) || seconds < 0) {
        return `invalid --timeout "${v}". Expected a number of seconds, e.g. 30.`;
      }
      f.timeoutSeconds = seconds;
      return undefined;
    },
  },
  {
    kind: "boolean",
    names: ["--verbose", "-v"],
    apply: (f) => {
      f.verbose = true;
    },
  },
  {
    kind: "boolean",
    names: ["--no-color"],
    apply: (f) => {
      f.noColor = true;
    },
  },
  {
    kind: "boolean",
    names: ["--yes", "-y"],
    apply: (f) => {
      f.yes = true;
    },
  },
  {
    kind: "boolean",
    names: ["--all"],
    apply: (f) => {
      f.all = true;
    },
  },
  {
    kind: "value",
    names: ["--columns"],
    valueName: "a,b,c",
    apply: (f, v) => {
      const cols = v
        .split(",")
        .map((c) => c.trim())
        .filter(Boolean);
      if (cols.length === 0) return `--columns needs at least one column name.`;
      f.columns = cols;
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--filter"],
    valueName: "field=value",
    apply: (f, v) => {
      if (!/[=~]/.test(v)) {
        return `invalid --filter "${v}". Expected field=value (exact) or field~value (contains).`;
      }
      f.filter = [...(f.filter ?? []), v];
      return undefined;
    },
  },
  {
    kind: "value",
    names: ["--sort"],
    valueName: "field",
    allowsLeadingDash: true,
    apply: (f, v) => {
      f.sort = v;
      return undefined;
    },
  },
  {
    kind: "boolean",
    names: ["--background"],
    apply: (f) => {
      f.background = true;
    },
  },
  {
    kind: "value",
    names: ["--file"],
    valueName: "path",
    apply: (f, v) => {
      if (!v) return "--file needs a path to a local file.";
      f.file = v;
      return undefined;
    },
  },
  {
    kind: "boolean",
    names: ["--help", "-h"],
    apply: (f) => {
      f.help = true;
    },
  },
  {
    kind: "boolean",
    names: ["--instructions"],
    apply: (f) => {
      f.instructions = true;
    },
  },
  {
    kind: "boolean",
    names: ["--version"],
    apply: (f) => {
      f.version = true;
    },
  },
];

const BY_NAME = new Map<string, GlobalSpec>();
for (const spec of GLOBALS) {
  for (const name of spec.names) BY_NAME.set(name, spec);
}

export function globalFlagNames(): string[] {
  return [...BY_NAME.keys()].sort();
}

export function parseGlobalFlags(argv: readonly string[]): ParsedArgv {
  const globals: GlobalFlags = {};
  const rest: string[] = [];
  const restFlagNames: string[] = [];
  const errors: string[] = [];
  let afterTerminator = false;

  for (let i = 0; i < argv.length; i++) {
    const token = argv[i] as string;

    if (afterTerminator) {
      rest.push(token);
      continue;
    }

    if (token === "--") {
      afterTerminator = true;
      continue;
    }

    const eq = token.indexOf("=");
    const inlineName =
      token.startsWith("--") && eq > 2 ? token.slice(0, eq) : null;
    const spec = BY_NAME.get(inlineName ?? token);
    if (!spec) {
      rest.push(token);

      if (token.startsWith("--")) {
        restFlagNames.push(inlineName ? inlineName.slice(2) : token.slice(2));
      }
      continue;
    }

    if (spec.kind === "boolean") {
      if (inlineName) {
        errors.push(`${inlineName} is a switch and takes no value.`);
        continue;
      }
      spec.apply(globals);
      continue;
    }

    let value: string | undefined;
    if (inlineName) {
      value = token.slice(eq + 1);
    } else {
      const next = argv[i + 1];
      const takeable =
        next !== undefined &&
        (!isFlagLike(next) || (spec.allowsLeadingDash && !BY_NAME.has(next)));
      if (takeable) {
        value = next;
        i++;
      }
    }
    if (value === undefined || value === "") {
      errors.push(
        `${spec.names[0]} needs a value: ${spec.names[0]} <${spec.valueName}>`,
      );
      continue;
    }
    const problem = spec.apply(globals, value);
    if (problem) errors.push(problem);
  }

  return { globals, rest, restFlagNames, errors };
}

function isFlagLike(token: string): boolean {
  return token.startsWith("-") && token !== "-";
}

export function validateVerbFlags(
  provided: readonly string[],
  known: readonly string[],
  command: string,
): string[] {
  const normalized = new Map<string, string>();
  for (const k of known) normalized.set(normalizeFlag(k), k);

  const errors: string[] = [];
  for (const flag of provided) {
    if (normalized.has(normalizeFlag(flag))) continue;
    const suggestion = closestMatch(flag, known);
    errors.push(
      suggestion
        ? `unknown flag --${flag} for \`${command}\`. Did you mean --${toFlag(suggestion)}?`
        : `unknown flag --${flag} for \`${command}\`. Run \`idapt help ${command}\` for its arguments.`,
    );
  }
  return errors;
}

function normalizeFlag(name: string): string {
  return name.replace(/[-_]/g, "").toLowerCase();
}

export function toFlag(field: string): string {
  return field.replace(/_/g, "-");
}

export function closestMatch(
  input: string,
  candidates: readonly string[],
): string | null {
  const needle = normalizeFlag(input);
  const maxDistance = Math.max(1, Math.floor(needle.length / 4));
  let best: string | null = null;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const candidate of candidates) {
    const distance = editDistance(needle, normalizeFlag(candidate));
    if (distance < bestDistance) {
      bestDistance = distance;
      best = candidate;
    }
  }
  return bestDistance <= maxDistance ? best : null;
}

export function editDistance(a: string, b: string): number {
  if (a === b) return 0;
  if (a.length === 0) return b.length;
  if (b.length === 0) return a.length;

  let twoBack: number[] = [];
  let previous = Array.from({ length: b.length + 1 }, (_, i) => i);
  for (let i = 1; i <= a.length; i++) {
    const current = new Array<number>(b.length + 1);
    current[0] = i;
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1;
      let value = Math.min(
        (current[j - 1] as number) + 1,
        (previous[j] as number) + 1,
        (previous[j - 1] as number) + cost,
      );
      if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) {
        value = Math.min(value, (twoBack[j - 2] as number) + 1);
      }
      current[j] = value;
    }
    twoBack = previous;
    previous = current;
  }
  return previous[b.length] as number;
}

export function flagNamesIn(argv: readonly string[]): string[] {
  const names: string[] = [];
  let afterTerminator = false;
  for (const token of argv) {
    if (afterTerminator) continue;
    if (token === "--") {
      afterTerminator = true;
      continue;
    }
    if (!token.startsWith("--")) continue;
    const eq = token.indexOf("=");
    names.push(eq > 2 ? token.slice(2, eq) : token.slice(2));
  }
  return names;
}
