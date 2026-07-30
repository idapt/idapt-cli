

import { commandCatalog } from "@idapt/api-contracts/v1/command-catalog.generated";
import { loadActiveCredentials } from "./auth/contexts";
import {
  type CommandSurface,
  commandsForResource,
  findCommand,
  listCommands,
  listResources,
  resolveCommandForCli,
  type V1CommandSpec,
} from "./catalog";
import { closestMatch, globalFlagNames } from "./flags";
import { renderHelp, renderInstructions } from "./help";
import { column } from "./text";
import { VERSION } from "./version";

const SURFACE: CommandSurface = "cli";

const GROUPS: readonly { title: string; resources: readonly string[] }[] = [
  {
    title: "Work",
    resources: ["chat", "agent", "drive", "notes", "tasks", "memory", "search"],
  },
  {
    title: "Compute",
    resources: ["computer", "browser-app", "functions", "repos", "ci"],
  },
  {
    title: "Models",
    resources: [
      "models",
      "inference",
      "image",
      "video",
      "audio",
      "custom-models",
      "ai-gateway",
      "provider-endpoint",
    ],
  },
  {
    title: "Automation",
    resources: [
      "automation",
      "hook",
      "skill",
      "integration",
      "web",
      "operation",
    ],
  },
  {
    title: "Data",
    resources: ["datastore", "blobs", "table", "realtime"],
  },
  {
    title: "Account",
    resources: [
      "me",
      "settings",
      "subscription",
      "workspace",
      "credential",
      "credential-binding",
      "api-key",
      "notification",
      "share",
      "shared-with-me",
      "help-center",
      "guide",
      "hub",
    ],
  },
];

const EXAMPLES: readonly [string, string][] = [
  ["idapt chat list", "your recent chats"],
  ["idapt drive read notes/todo.md", "read a file by path"],
  [
    "idapt computer exec dev-box --command 'uptime'",
    "run a command on a machine",
  ],
  ["idapt agent list -o json", "machine-readable output"],
  ["idapt workspace use acme/api", "pin every later command to a workspace"],
  ["idapt help computer", "what a resource can do"],
];

const CLIENT_COMMANDS: readonly [string, string][] = [
  ["login | logout | whoami", "Sign in, sign out, show who you are"],
  ["auth list | switch | rename | remove", "Work with several accounts"],
  ["workspace use | current | clear", "Pin the workspace commands run in"],
  ["agent use | current | clear", "Act as one of your agents"],
  ["computer shell <computer>", "Interactive shell on a machine"],
  ["app init | deploy", "Scaffold and ship a browser-app"],
  ["config list | get | set | path", "CLI settings"],
  ["open [<type> <id>]", "Open the web app here"],
  ["completion <shell>", "Shell tab-completion"],
  ["version | upgrade", "Version, and update to the latest"],
];

export interface HelpDocOptions {
  readonly env?: Record<string, string | undefined>;

  readonly unknownIsError?: boolean;
}

function dumpJson(resource?: string): string {
  const entries = resource
    ? commandCatalog.filter((c) => c.resource === resource)
    : commandCatalog;
  return JSON.stringify(entries, null, 2);
}

function hiddenByFeatureFlag(): (spec: { featureFlag?: string }) => boolean {
  let enabled: readonly string[] | undefined;
  try {
    enabled = loadActiveCredentials().enabledFeatures;
  } catch {
    enabled = undefined;
  }
  if (!enabled) return () => false;
  const allowed = new Set(enabled);
  return (spec) => Boolean(spec.featureFlag && !allowed.has(spec.featureFlag));
}

function groupedResources(): { title: string; resources: string[] }[] {
  const hidden = hiddenByFeatureFlag();
  const available = new Set(
    listCommands(SURFACE)
      .filter((c) => !hidden(c))
      .map((c) => c.resource),
  );
  const placed = new Set<string>();
  const groups = GROUPS.map((g) => {
    const resources = g.resources.filter((r) => available.has(r));
    for (const r of resources) placed.add(r);
    return { title: g.title, resources };
  }).filter((g) => g.resources.length > 0);

  const leftover = [...available].filter((r) => !placed.has(r)).sort();
  if (leftover.length > 0) groups.push({ title: "More", resources: leftover });
  return groups;
}

function helpIndex(): string {
  const lines: string[] = [
    `idapt ${VERSION} - your AI workspace, from the terminal`,
    "",
    "Usage:",
    "  idapt <resource> <verb> [args]",
    "",
    "Examples:",
    ...column(
      EXAMPLES.map(([cmd, what]) => [`  ${cmd}`, what]),
      "  ",
    ),
    "",
    "Resources:",
  ];

  for (const group of groupedResources()) {
    lines.push(`  ${group.title}`, `    ${group.resources.join(", ")}`);
  }

  lines.push(
    "",
    "Commands:",
    ...column(
      CLIENT_COMMANDS.map(([cmd, what]) => [`  idapt ${cmd}`, what]),
      "  ",
    ),
    "",
    "Discover:",
    ...column(
      [
        ["  idapt help <resource>", "list a resource's verbs"],
        ["  idapt help <resource> <verb>", "one command's full contract"],
        ["  idapt instructions <resource>", "when and why to use it"],
      ],
      "  ",
    ),
    "",
    "Global flags:",
    ...column(
      [
        [
          "  -o, --output <table|json|jsonl|quiet>",
          "output format (TTY table, pipe json)",
        ],
        ["  -q, --quiet", "ids only"],
        ["  -w, --workspace <workspace>", "workspace to operate in"],
        ["  --agent <agent>", "act as this agent"],
        ["  --context <name>", "which stored account to use"],
        ["  -y, --yes", "skip confirmation prompts"],
        ["  --all", "walk every page of a list"],
        ["  --columns / --filter / --sort", "shape list output"],
        ["  --background", "return long-running work immediately"],
        ["  --timeout <seconds>", "per-request timeout"],
        ["  -v, --verbose", "trace HTTP to stderr"],
        [
          "  --api-key <key> / --api-url <url>",
          "connection (or IDAPT_API_KEY / IDAPT_API_URL)",
        ],
      ],
      "  ",
    ),
    "",
    "Docs: https://idapt.app/cli",
  );
  return lines.join("\n");
}

function resourceHelp(
  resource: string,
  cmds: readonly V1CommandSpec[],
): string {
  const rows = cmds.map(
    (c) => [`  ${c.verb}`, c.summary ?? ""] as [string, string],
  );
  const gated = cmds.find((c) => c.featureFlag)?.featureFlag;
  return [
    `idapt ${resource} - commands`,
    ...(gated
      ? ["", "  (preview: this product is not enabled for every account yet)"]
      : []),
    "",
    ...column(rows, "  "),
    "",
    `Run \`idapt help ${resource} <verb>\` for a command's full contract,`,
    `or \`idapt instructions ${resource}\` for the playbook.`,
  ].join("\n");
}

export function renderHelpDoc(
  tokens: readonly string[],
  options: HelpDocOptions = {},
): string {
  const positional = tokens.filter((t) => !t.startsWith("-"));
  if (tokens.includes("--dump-json")) return dumpJson(positional[0]);
  if (positional.length === 0) return helpIndex();

  const resources = listResources(SURFACE);
  const resource = positional[0] as string;

  if (positional.length === 1) {
    const cmds = commandsForResource(resource, SURFACE);
    if (cmds.length > 0) return resourceHelp(resource, cmds);
    const suggestion = closestMatch(resource, [
      ...resources,
      ...globalFlagNames(),
    ]);
    const headline = suggestion
      ? `Unknown resource "${resource}". Did you mean "${suggestion}"?`
      : `Unknown resource "${resource}".`;
    return options.unknownIsError
      ? `${headline}\n\nRun \`idapt help\` to see everything.`
      : `${headline}\n\n${helpIndex()}`;
  }

  const resolved = resolveCommandForCli(positional);
  const spec =
    resolved?.spec ?? findCommand(`${positional[0]} ${positional[1]}`);
  if (spec) return renderHelp(spec);

  const cmds = commandsForResource(resource, SURFACE);
  if (cmds.length > 0) {
    const verb = positional[1] as string;
    const suggestion = closestMatch(
      verb,
      cmds.map((c) => c.verb),
    );
    const headline = suggestion
      ? `Unknown command "${resource} ${verb}". Did you mean "${resource} ${suggestion}"?`
      : `Unknown command "${resource} ${verb}".`;
    return `${headline}\n\n${resourceHelp(resource, cmds)}`;
  }
  return `Unknown command "${positional.slice(0, 2).join(" ")}".\n\n${helpIndex()}`;
}

export function renderInstructionsDoc(tokens: readonly string[]): string {
  if (tokens.length === 0) {
    return [
      "idapt instructions - resource playbooks",
      "",
      "Instructions are RESOURCE-SCOPED. Run `idapt instructions <resource>`:",
      `  ${listResources(SURFACE).join(", ")}`,
    ].join("\n");
  }
  return renderInstructions(tokens[0] as string);
}
