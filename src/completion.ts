

import { commandsForResource, listResources } from "./catalog";

export const COMPLETION_SHELLS = ["bash", "zsh", "fish", "powershell"] as const;
export type CompletionShell = (typeof COMPLETION_SHELLS)[number];

const EXTRA_TOP_LEVEL = [
  "login",
  "logout",
  "whoami",
  "auth",
  "app",
  "help",
  "instructions",
  "upgrade",
  "version",
  "completion",
];

const EXTRA_SUBCOMMANDS: Record<string, readonly string[]> = {
  auth: ["login", "logout", "status", "switch", "list", "rename", "remove"],
  app: ["init", "deploy"],
  workspace: ["use", "current", "clear"],
  agent: ["use", "current", "clear"],
  computer: ["shell"],
  completion: [...COMPLETION_SHELLS, "install"],
};

export function completeWords(words: readonly string[]): string[] {
  const toComplete = words.length > 0 ? (words[words.length - 1] ?? "") : "";
  const prior = words.slice(0, -1);

  if (prior.length === 0) {
    const candidates = [...new Set([...listResources(), ...EXTRA_TOP_LEVEL])];
    return filterSorted(candidates, toComplete);
  }

  if (prior.length === 1) {
    const resource = prior[0] as string;
    const verbs = commandsForResource(resource).map((c) =>
      c.command.slice(resource.length + 1),
    );
    const extras = EXTRA_SUBCOMMANDS[resource] ?? [];
    return filterSorted([...new Set([...verbs, ...extras])], toComplete);
  }

  return filterSorted(
    [
      "--json",
      "--output",
      "--workspace",
      "--agent",
      "--help",
      "--instructions",
    ],
    toComplete,
  );
}

function filterSorted(candidates: string[], toComplete: string): string[] {
  const matches = toComplete
    ? candidates.filter((c) => c.startsWith(toComplete))
    : candidates;
  return [...matches].sort();
}

export function completionScript(shell: CompletionShell): string {
  switch (shell) {
    case "bash":
      return `# idapt bash completion
_idapt_complete() {
  local IFS=$'\\n'
  COMPREPLY=( $(idapt __complete "\${COMP_WORDS[@]:1}" 2>/dev/null) )
}
complete -o default -F _idapt_complete idapt
`;
    case "zsh":
      return `#compdef idapt
# idapt zsh completion
_idapt() {
  local -a completions
  completions=(\${(f)"$(idapt __complete \${words[2,$CURRENT]} 2>/dev/null)"})
  compadd -- \${completions}
}
compdef _idapt idapt
`;
    case "fish":
      return `# idapt fish completion
function __idapt_complete
  set -l tokens (commandline -opc) (commandline -ct)
  idapt __complete $tokens[2..-1] 2>/dev/null
end
complete -c idapt -f -a "(__idapt_complete)"
`;
    case "powershell":
      return `# idapt PowerShell completion
Register-ArgumentCompleter -Native -CommandName idapt -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $words = $commandAst.CommandElements | Select-Object -Skip 1 | ForEach-Object { $_.ToString() }
  idapt __complete @words 2>$null | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`;
  }
}

export function completionInstallHint(shell: CompletionShell): string {
  switch (shell) {
    case "bash":
      return 'Add to ~/.bashrc:\n  eval "$(idapt completion bash)"';
    case "zsh":
      return 'Add to ~/.zshrc:\n  eval "$(idapt completion zsh)"';
    case "fish":
      return "Write it once:\n  idapt completion fish > ~/.config/fish/completions/idapt.fish";
    case "powershell":
      return "Add to $PROFILE:\n  idapt completion powershell | Out-String | Invoke-Expression";
  }
}

export function detectShell(
  env: Record<string, string | undefined>,
): CompletionShell {
  const shell = env.SHELL ?? "";
  if (shell.includes("zsh")) return "zsh";
  if (shell.includes("fish")) return "fish";
  if (env.PSModulePath) return "powershell";
  return "bash";
}
