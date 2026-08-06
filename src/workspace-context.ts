

import { loadActiveCredentials, saveActiveCredentials } from "./auth/contexts";
import { execute } from "./execute";
import { EXIT_ERROR, EXIT_NOT_FOUND } from "./exit-codes";
import { column } from "./text";
import type { AgentToolsTransport } from "./transport";
import { effectiveWorkspace, originLabel } from "./workspace-ref";

export interface WorkspaceContextIo {
  print: (s: string) => void;
  err: (s: string) => void;

  env?: Record<string, string | undefined>;
}

function pick(
  row: Record<string, unknown>,
  ...keys: string[]
): string | undefined {
  for (const k of keys) {
    const v = row[k];
    if (typeof v === "string" && v.length > 0) return v;
  }
  return undefined;
}

export function qualifiedSlug(
  row: Record<string, unknown>,
): string | undefined {
  const q = pick(row, "qualified_slug", "qualifiedSlug");
  if (q) return q;
  const owner = pick(row, "owner_slug", "ownerSlug", "owner");
  const slug = pick(row, "slug");
  return owner && slug ? `${owner}/${slug}` : slug;
}

function resourceIdOf(row: Record<string, unknown>): string | undefined {
  return pick(row, "resourceId", "resource_id", "id");
}

function displayName(row: Record<string, unknown>): string {
  return (
    pick(row, "name") ?? qualifiedSlug(row) ?? resourceIdOf(row) ?? "(unnamed)"
  );
}

function isDefault(row: Record<string, unknown>): boolean {
  return row.is_default === true || row.isDefault === true;
}

function matches(row: Record<string, unknown>, ref: string): boolean {
  const r = ref.toLowerCase();
  return [
    pick(row, "resourceId", "resource_id"),
    pick(row, "id"),
    pick(row, "slug"),
    qualifiedSlug(row),
  ].some((c) => c !== undefined && c.toLowerCase() === r);
}

let workspaceListCache:
  | Promise<Record<string, unknown>[] | { error: string }>
  | undefined;

async function listWorkspaces(
  transport: AgentToolsTransport,
): Promise<Record<string, unknown>[] | { error: string }> {
  workspaceListCache ??= (async () => {
    const listed = await execute("idapt workspace list", {
      transport,
      mode: "json",
    });
    if (!listed.ok) {
      return { error: listed.error ?? "could not list workspaces" };
    }
    return Array.isArray(listed.data)
      ? (listed.data as Record<string, unknown>[])
      : [];
  })();
  const result = await workspaceListCache;

  if ("error" in result) workspaceListCache = undefined;
  return result;
}

export function resetWorkspaceListCache(): void {
  workspaceListCache = undefined;
}

export type WorkspaceLookup =
  | { ok: true; resourceId: string; label: string; qualified?: string }
  | { ok: false; error: string; code: number };

export async function resolveWorkspaceRef(
  transport: AgentToolsTransport,
  ref: string,
): Promise<WorkspaceLookup> {
  const rows = await listWorkspaces(transport);
  if ("error" in rows) {
    return {
      ok: false,
      error: `idapt: could not resolve workspace "${ref}": ${rows.error}`,
      code: EXIT_ERROR,
    };
  }

  const exact = rows.filter((w) => matches(w, ref));
  if (exact.length === 1) return toLookup(exact[0] as Record<string, unknown>);

  const bare = rows.filter(
    (w) => (pick(w, "slug") ?? "").toLowerCase() === ref.toLowerCase(),
  );
  if (bare.length === 1) return toLookup(bare[0] as Record<string, unknown>);
  if (bare.length > 1) {
    const candidates = bare
      .map((w) => `  ${qualifiedSlug(w) ?? resourceIdOf(w)}`)
      .join("\n");
    return {
      ok: false,
      error: `idapt: "${ref}" matches more than one workspace. Use the qualified form:\n${candidates}`,
      code: EXIT_ERROR,
    };
  }

  const available =
    column(
      rows.map((w) => [
        `  ${qualifiedSlug(w) ?? resourceIdOf(w)}`,
        displayName(w),
      ]),
    ).join("\n") || "  (none)";
  return {
    ok: false,
    error: `idapt: no workspace matches "${ref}". Yours:\n${available}`,
    code: EXIT_NOT_FOUND,
  };
}

function toLookup(row: Record<string, unknown>): WorkspaceLookup {
  const resourceId = resourceIdOf(row);
  if (!resourceId) {
    return {
      ok: false,
      error: "idapt: the matched workspace has no id.",
      code: EXIT_ERROR,
    };
  }
  const qualified = qualifiedSlug(row);
  return {
    ok: true,
    resourceId,
    label: displayName(row),
    ...(qualified ? { qualified } : {}),
  };
}

async function describeDefault(
  transport: AgentToolsTransport | undefined,
): Promise<string> {
  if (!transport) return "your default workspace";
  const rows = await listWorkspaces(transport);
  if ("error" in rows) return "your default workspace";
  const fallback = rows.find(isDefault);
  return fallback
    ? `your default workspace (${displayName(fallback)})`
    : "your default workspace";
}

export async function runWorkspaceContext(
  sub: string,
  args: readonly string[],

  transport: AgentToolsTransport | undefined,
  io: WorkspaceContextIo,
): Promise<number> {
  const creds = loadActiveCredentials();

  if (sub === "current") {
    const effective = effectiveWorkspace({
      env: io.env,
      pin: creds.defaultWorkspaceSlug ?? creds.defaultWorkspaceId,
    });
    if (effective) {

      io.print(`${effective.ref}\n`);
      if (effective.origin !== "pin") {
        io.err(`  (from ${originLabel(effective.origin)})\n`);
      }
    } else {
      io.print(
        `No workspace pinned. Commands run in ${await describeDefault(transport)}.\n`,
      );
    }
    return 0;
  }

  if (sub === "clear") {
    if (!creds.defaultWorkspaceId) {
      io.print("No workspace was pinned.\n");
      return 0;
    }
    const was = creds.defaultWorkspaceSlug ?? creds.defaultWorkspaceId;
    saveActiveCredentials({
      ...creds,
      defaultWorkspaceId: undefined,
      defaultWorkspaceSlug: undefined,
      previousWorkspaceId: creds.defaultWorkspaceId,
      previousWorkspaceSlug: creds.defaultWorkspaceSlug,
    });
    io.print(
      `Unpinned ${was}. Commands now run in ${await describeDefault(transport)}.\n`,
    );
    return 0;
  }

  const ref = args[0];
  if (!transport) {
    io.err("idapt workspace use: not signed in. Run `idapt login` first.\n");
    return EXIT_ERROR;
  }

  if (!ref) {
    const rows = await listWorkspaces(transport);
    if ("error" in rows) {
      io.err(`idapt workspace use: ${rows.error}\n`);
      return EXIT_ERROR;
    }
    io.print(
      [
        "Which workspace? Run `idapt workspace use <workspace>`:",
        ...rows.map((w) => {
          const current = resourceIdOf(w) === creds.defaultWorkspaceId;
          return `${current ? "*" : " "} ${qualifiedSlug(w) ?? resourceIdOf(w)}  ${displayName(w)}`;
        }),
        "",
      ].join("\n"),
    );
    return 0;
  }

  const target =
    ref === "-"
      ? (creds.previousWorkspaceId ?? creds.previousWorkspaceSlug)
      : ref;
  if (!target) {
    io.err("idapt workspace use: no previous workspace to go back to.\n");
    return EXIT_ERROR;
  }

  const lookup = await resolveWorkspaceRef(transport, target);
  if (!lookup.ok) {
    io.err(`${lookup.error}\n`);
    return lookup.code;
  }

  saveActiveCredentials({
    ...creds,
    defaultWorkspaceId: lookup.resourceId,
    defaultWorkspaceSlug: lookup.qualified ?? lookup.resourceId,
    previousWorkspaceId: creds.defaultWorkspaceId,
    previousWorkspaceSlug: creds.defaultWorkspaceSlug,

    defaultAgentMemoryBoxId: undefined,
  });
  io.print(
    `Now working in ${lookup.label}${lookup.qualified ? ` (${lookup.qualified})` : ""}.\n` +
      "Override per command with -w, or run `idapt workspace clear`.\n",
  );
  return 0;
}
