

interface ClientHelpEntry {

  readonly path: readonly string[];
  readonly usage: string;
}

const ENTRIES: readonly ClientHelpEntry[] = [
  {
    path: ["login"],
    usage: [
      "idapt login [--device] [--api-key-stdin] [--context <name>]",
      "",
      "  Sign in and store the credential for later commands.",
      "",
      "  (default)          Open a browser and approve the sign-in there.",
      "  --device           Show a code to enter on another machine. Use this",
      "                     over SSH or anywhere without a browser.",
      "  --api-key-stdin    Read an API key from stdin and store it:",
      "                       printf '%s' \"$KEY\" | idapt login --api-key-stdin",
      "  --context <name>   Sign in as an ADDITIONAL account under this name,",
      "                     leaving the current one in place.",
      "",
      "  Signing in again for an existing account replaces its credential.",
      "  See where you are signed in with `idapt whoami`.",
    ].join("\n"),
  },
  {
    path: ["logout"],
    usage: [
      "idapt logout [--context <name>]",
      "",
      "  Forget the stored credential for an account. Other accounts are kept;",
      "  list them with `idapt auth list`.",
    ].join("\n"),
  },
  {
    path: ["whoami"],
    usage: [
      "idapt whoami [-o json] [--context <name>]",
      "",
      "  Show who you are signed in as, which workspace and agent commands will",
      "  use, and where the credential came from.",
      "",
      "  -o json    Emit the same facts as an object, for scripts.",
    ].join("\n"),
  },
  {
    path: ["upgrade"],
    usage: [
      "idapt upgrade [--check] [--next]",
      "",
      "  Update this CLI to the latest published version.",
      "",
      "  --check    Report whether an update exists; change nothing.",
      "  --next     Take the pre-release channel instead of stable.",
    ].join("\n"),
  },
  {
    path: ["auth"],
    usage: [
      "idapt auth <command>",
      "",
      "  login | logout | status      Same as `idapt login` / `logout` / `whoami`.",
      "  list                         List stored accounts; * marks the active one.",
      "  switch <name>                Make an account active.",
      "  rename <from> <to>           Rename an account.",
      "  remove <name>                Forget an account.",
      "",
      "  -o json    Machine-readable form of `list`.",
      "",
      "  An account is a named credential. Use one per identity or per server,",
      "  and select it per command with --context <name> or IDAPT_CONTEXT.",
    ].join("\n"),
  },
  {
    path: ["config"],
    usage: [
      "idapt config <command>",
      "",
      "  list           Show the effective settings and where each came from.",
      "  get <key>      Print one value.",
      "  set <key> <v>  Store a value.",
      "  unset <key>    Remove a stored value.",
      "  path           Print the config file path.",
      "",
      "  -o json    Machine-readable form of `list`.",
    ].join("\n"),
  },
  {
    path: ["completion"],
    usage: [
      "idapt completion <bash|zsh|fish|powershell>",
      "",
      "  Print the shell completion script. Add it to your shell profile, or run",
      "  `idapt completion install` for the one-liner for your shell.",
    ].join("\n"),
  },
  {
    path: ["app"],
    usage: [
      "idapt app <command>",
      "",
      "  init [dir]     Scaffold a browser-app in dir (default: current dir).",
      "  deploy [dir]   Upload a built dir and deploy it.",
      "",
      "  --app <id>     Deploy to an existing app instead of creating one.",
      "  -w <workspace> Workspace to create the app in.",
    ].join("\n"),
  },
  {
    path: ["computer", "shell"],
    usage: [
      "idapt computer shell <computer> [-w <workspace>]",
      "",
      "  Open an interactive shell on a paired computer. Needs a terminal;",
      "  for one-off commands in a script use `idapt computer exec` instead.",
    ].join("\n"),
  },
  {
    path: ["workspace", "use"],
    usage: [
      "idapt workspace use [<workspace>|-]",
      "",
      "  Pin the workspace that later commands run in. With no argument, list",
      "  the choices. `-` goes back to the previously pinned workspace.",
      "",
      "  Also: `idapt workspace current` (show it), `idapt workspace clear`",
      "  (unpin). Override for one command with -w, or for one shell with",
      "  IDAPT_WORKSPACE.",
    ].join("\n"),
  },
  {
    path: ["agent", "use"],
    usage: [
      "idapt agent use [<agent>]",
      "",
      "  Act as one of your agents: `idapt chat create` binds it and",
      "  `idapt memory ...` reads and writes its Memory.",
      "",
      "  Also: `idapt agent current`, `idapt agent clear`. Override for one",
      "  command with --agent.",
    ].join("\n"),
  },
  {
    path: ["uninstall"],
    usage: [
      "idapt uninstall",
      "",
      "  Print how to remove the CLI and where its stored data lives. Removes",
      "  nothing itself.",
    ].join("\n"),
  },
];

export function clientHelpFor(tokens: readonly string[]): string | null {
  let best: ClientHelpEntry | null = null;
  for (const entry of ENTRIES) {
    if (entry.path.length > tokens.length) continue;
    if (!entry.path.every((token, i) => tokens[i] === token)) continue;
    if (!best || entry.path.length > best.path.length) best = entry;
  }
  return best?.usage ?? null;
}

export function clientCommandPaths(): string[][] {
  return ENTRIES.map((e) => [...e.path]);
}
