

import {
  agentExposedCommands,
  cliExposedCommands,
  getResourcePlaybook,
  type V1CommandSpec,
  v1CommandRegistry,
} from "@idapt/api-contracts/v1/contracts";
import { COLLAPSED_OPS, reconcileToV1 } from "./reconcile";

export type CommandSurface = "cli" | "agent";

function commandsFor(surface: CommandSurface): readonly V1CommandSpec[] {
  return surface === "cli" ? cliExposedCommands() : agentExposedCommands();
}

export function listCommands(
  surface: CommandSurface,
): readonly V1CommandSpec[] {
  return commandsFor(surface);
}

export function listResources(surface: CommandSurface): string[] {
  return [...new Set(commandsFor(surface).map((c) => c.resource))].sort();
}

export function commandsForResource(
  resource: string,
  surface: CommandSurface,
): readonly V1CommandSpec[] {
  return commandsFor(surface).filter((c) => c.resource === resource);
}

export function findCommand(command: string): V1CommandSpec | undefined {
  return v1CommandRegistry.find((c) => c.command === command);
}

export function resolveCommand(
  pathTokens: readonly string[],
  surface: CommandSurface,
): { spec: V1CommandSpec; rest: string[] } | undefined {
  const exposed = new Set(commandsFor(surface).map((c) => c.command));
  for (let n = pathTokens.length; n >= 1; n--) {
    const candidate = pathTokens.slice(0, n).join(" ");
    if (!exposed.has(candidate)) continue;
    const spec = findCommand(candidate);
    if (spec) return { spec, rest: pathTokens.slice(n) };
  }
  return undefined;
}

const OPS_BY_COMMAND: ReadonlyMap<string, ReadonlySet<string>> = (() => {
  const map = new Map<string, Set<string>>();
  for (const [agentPath, op] of Object.entries(COLLAPSED_OPS)) {
    const v1 = reconcileToV1(agentPath);
    const set = map.get(v1) ?? new Set<string>();
    set.add(op);
    map.set(v1, set);
  }
  return map;
})();

function opsForCommand(command: string): ReadonlySet<string> | undefined {
  return OPS_BY_COMMAND.get(command);
}

const VERB_ALIASES: Record<string, string> = {
  ls: "list",
  rm: "delete",
  del: "delete",
  mv: "move",
  cp: "copy",
  info: "get",
  show: "get",
};

export function resolveCommandForCli(pathTokens: readonly string[]):
  | {
      spec: V1CommandSpec;
      rest: string[];

      impliedArgs: Record<string, unknown>;
    }
  | undefined {
  const exposed = new Set(cliExposedCommands().map((c) => c.command));

  const attempt = (
    tokens: readonly string[],
    consumed: number,
    impliedArgs: Record<string, unknown> = {},
  ) => {
    for (let n = Math.min(tokens.length, 3); n >= 1; n--) {
      const candidate = tokens.slice(0, n).join(" ");
      if (!exposed.has(candidate)) continue;
      const spec = findCommand(candidate);
      if (!spec) continue;
      return {
        spec,
        rest: [...pathTokens.slice(consumed + n)],
        impliedArgs,
      };
    }
    return undefined;
  };

  const direct = attempt(pathTokens, 0);
  if (direct) {

    const ops = opsForCommand(direct.spec.command);
    const next = direct.rest[0];
    if (ops && next && ops.has(next)) {
      return {
        spec: direct.spec,
        rest: direct.rest.slice(1),
        impliedArgs: { op: next },
      };
    }
    return direct;
  }

  if (pathTokens.length >= 2) {
    const alias = VERB_ALIASES[pathTokens[1] as string];
    if (alias) {
      const aliased = attempt([pathTokens[0] as string, alias], 0);

      if (aliased) return { ...aliased, rest: [...pathTokens.slice(2)] };
    }
  }

  for (const n of [3, 2]) {
    if (pathTokens.length < n) continue;
    const agentPath = pathTokens.slice(0, n).join(" ");

    const hyphenated =
      n === 3
        ? `${pathTokens[0]} ${pathTokens[1]}-${pathTokens[2]}`
        : agentPath;
    for (const shape of new Set([agentPath, hyphenated])) {
      const v1 = reconcileToV1(shape);
      if (v1 === shape) continue;
      const spec = findCommand(v1);
      if (!spec || !exposed.has(v1)) continue;
      const op = COLLAPSED_OPS[shape];
      return {
        spec,
        rest: [...pathTokens.slice(n)],
        impliedArgs: op ? { op } : {},
      };
    }
  }

  return undefined;
}

export { getResourcePlaybook };
export type { V1CommandSpec };
