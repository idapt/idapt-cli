

import {
  agentExposedCommands,
  getResourcePlaybook,
  type V1CommandSpec,
} from "@idapt/api-contracts/v1/contracts";

export function listCommands(): readonly V1CommandSpec[] {
  return agentExposedCommands();
}

export function listResources(): string[] {
  return [...new Set(agentExposedCommands().map((c) => c.resource))].sort();
}

export function commandsForResource(
  resource: string,
): readonly V1CommandSpec[] {
  return agentExposedCommands().filter((c) => c.resource === resource);
}

export function findCommand(command: string): V1CommandSpec | undefined {
  return agentExposedCommands().find((c) => c.command === command);
}

export function resolveCommand(
  pathTokens: readonly string[],
): { spec: V1CommandSpec; rest: string[] } | undefined {
  for (let n = pathTokens.length; n >= 1; n--) {
    const candidate = pathTokens.slice(0, n).join(" ");
    const spec = findCommand(candidate);
    if (spec) return { spec, rest: pathTokens.slice(n) };
  }
  return undefined;
}

export { getResourcePlaybook };
export type { V1CommandSpec };
