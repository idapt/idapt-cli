

import { loadCredentials, saveCredentials } from "./auth/credentials";
import { execute } from "./execute";
import type { AgentToolsTransport } from "./transport";

export interface WorkspaceContextIo {
  print: (s: string) => void;
  err: (s: string) => void;
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

function qualifiedSlug(row: Record<string, unknown>): string | undefined {
  const q = pick(row, "qualified_slug", "qualifiedSlug");
  if (q) return q;
  const owner = pick(row, "owner_slug", "ownerSlug", "owner");
  const slug = pick(row, "slug");
  return owner && slug ? `${owner}/${slug}` : slug;
}

function resourceIdOf(row: Record<string, unknown>): string | undefined {
  return pick(row, "resourceId", "resource_id", "id");
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

export async function runWorkspaceContext(
  sub: string,
  args: readonly string[],

  transport: AgentToolsTransport | undefined,
  io: WorkspaceContextIo,
): Promise<number> {
  if (sub === "current") {
    const creds = loadCredentials();
    if (creds.defaultWorkspaceId) {
      io.print(`${creds.defaultWorkspaceSlug ?? creds.defaultWorkspaceId}\n`);
    } else {
      io.print(
        "(no default workspace — scope verbs run in your personal workspace)\n",
      );
    }
    return 0;
  }

  if (sub === "clear") {
    const creds = loadCredentials();
    if (!creds.defaultWorkspaceId) {
      io.print("No default workspace was set.\n");
      return 0;
    }
    const was = creds.defaultWorkspaceSlug ?? creds.defaultWorkspaceId;
    creds.defaultWorkspaceId = undefined;
    creds.defaultWorkspaceSlug = undefined;
    saveCredentials(creds);
    io.print(
      `Cleared the default workspace (was ${was}). Scope verbs now run in your personal workspace.\n`,
    );
    return 0;
  }

  const ref = args[0];
  if (!ref) {
    io.err(
      "idapt workspace use: missing workspace. Usage: `idapt workspace use <ownerSlug/workspaceSlug | resourceId>`.\n",
    );
    return 1;
  }
  if (!transport) {
    io.err("idapt workspace use: not logged in. Run `idapt login` first.\n");
    return 1;
  }

  const listed = await execute("idapt workspace list", {
    transport,
    mode: "json",
  });
  if (!listed.ok) {
    io.err(
      `idapt workspace use: could not list workspaces: ${listed.error ?? "unknown error"}\n`,
    );
    return 1;
  }
  const rows = Array.isArray(listed.data)
    ? (listed.data as Record<string, unknown>[])
    : [];
  const match = rows.find((w) => matches(w, ref));
  if (!match) {
    const available =
      rows
        .map((w) => `  ${qualifiedSlug(w) ?? resourceIdOf(w) ?? "?"}`)
        .join("\n") || "  (none)";
    io.err(
      `idapt workspace use: no workspace matches "${ref}". Your workspaces:\n${available}\n`,
    );
    return 1;
  }

  const resourceId = resourceIdOf(match);
  if (!resourceId) {
    io.err(
      "idapt workspace use: the matched workspace is missing a resourceId.\n",
    );
    return 1;
  }
  const creds = loadCredentials();
  creds.defaultWorkspaceId = resourceId;
  creds.defaultWorkspaceSlug = qualifiedSlug(match) ?? resourceId;
  saveCredentials(creds);
  io.print(
    `Default workspace set to ${creds.defaultWorkspaceSlug}. Scope verbs now run there (override per-call with --workspace-id, or run \`idapt workspace clear\`).\n`,
  );
  return 0;
}
