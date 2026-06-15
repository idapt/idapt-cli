
import {
  commandsForResource,
  findCommand,
  listResources,
  type V1CommandSpec,
} from "./catalog";
import { renderHelp, renderInstructions } from "./help";
import { VERSION } from "./version";

function helpIndex(): string {
  const resources = listResources().sort().join(", ");
  return [
    `idapt ${VERSION} — your AI workspace, from the terminal`,
    "",
    "Usage:",
    "  idapt <resource> <verb> [args]     Run a command (e.g. idapt drive list)",
    "  idapt help [resource [verb]]       Show a command's contract",
    "  idapt instructions [resource]      Show a resource's playbook",
    "  idapt login | logout | whoami      Authenticate",
    "  idapt upgrade                      Update to the latest version",
    "",
    "Global flags:",
    "  -o, --output <table|json|jsonl|quiet>   Output format (TTY→table, pipe→json)",
    "  --api-key <key>                          Bearer token (or IDAPT_API_KEY env)",
    "  --api-url <url>                           API base URL (or IDAPT_API_URL env)",
    "",
    "Resources:",
    `  ${resources}`,
    "",
    "Auth: browser sign-in (`idapt login`) or paste a key (IDAPT_API_KEY).",
    "Docs: https://idapt.app/cli",
  ].join("\n");
}

function resourceHelp(
  resource: string,
  cmds: readonly V1CommandSpec[],
): string {
  const rows = cmds
    .map((c) => `  ${c.verb.padEnd(16)} ${c.summary ?? ""}`.trimEnd())
    .join("\n");
  return [
    `idapt ${resource} — commands`,
    "",
    rows,
    "",
    `Run \`idapt help ${resource} <verb>\` for a command's full contract,`,
    `or \`idapt instructions ${resource}\` for the playbook.`,
  ].join("\n");
}

export function renderHelpDoc(tokens: readonly string[]): string {
  if (tokens.length === 0) return helpIndex();
  if (tokens.length === 1) {
    const resource = tokens[0];
    const cmds = commandsForResource(resource);
    if (cmds.length === 0) {
      return `Unknown resource "${resource}".\n\n${helpIndex()}`;
    }
    return resourceHelp(resource, cmds);
  }

  const command = `${tokens[0]} ${tokens[1]}`;
  const spec = findCommand(command);
  if (spec) return renderHelp(spec);
  const cmds = commandsForResource(tokens[0]);
  if (cmds.length > 0) {
    return `Unknown command "${command}".\n\n${resourceHelp(tokens[0], cmds)}`;
  }
  return `Unknown command "${command}".\n\n${helpIndex()}`;
}

export function renderInstructionsDoc(tokens: readonly string[]): string {
  if (tokens.length === 0) {
    return [
      "idapt instructions — resource playbooks",
      "",
      "Instructions are RESOURCE-SCOPED. Run `idapt instructions <resource>`:",
      `  ${listResources().sort().join(", ")}`,
    ].join("\n");
  }
  return renderInstructions(tokens[0]);
}
