
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";
import type { ExecuteResult } from "../execute";

export const MAX_BUNDLE_FILES = 500;
export const MAX_BUNDLE_FILE_BYTES = 25 * 1024 * 1024;
export const MAX_BUNDLE_TOTAL_BYTES = 100 * 1024 * 1024;

export interface CollectedFile {

  readonly path: string;
  readonly bytes: Uint8Array;
}

export interface DeployFile {
  readonly path: string;
  readonly content_b64: string;
}

export interface DeployFsIo {

  readonly collect: (dir: string) => CollectedFile[];
}

export type V1Exec = (
  commandName: string,
  args: Record<string, unknown>,
) => Promise<ExecuteResult>;

export interface DeployOptions {

  readonly dir: string;

  readonly app?: string;

  readonly workspace?: string;

  readonly name?: string;
}

export interface DeployResult {
  readonly ok: boolean;

  readonly written?: number;

  readonly appId?: string;

  readonly deploymentId?: string;

  readonly url?: string;

  readonly created?: boolean;
  readonly error?: string;
}

export function parseDeployArgs(args: readonly string[]): DeployOptions {
  let dir: string | undefined;
  let app: string | undefined;
  let workspace: string | undefined;
  let name: string | undefined;

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--app") app = args[++i];
    else if (a.startsWith("--app=")) app = a.slice("--app=".length);
    else if (a === "--workspace" || a === "--workspace-id")
      workspace = args[++i];
    else if (a.startsWith("--workspace="))
      workspace = a.slice("--workspace=".length);
    else if (a.startsWith("--workspace-id="))
      workspace = a.slice("--workspace-id=".length);
    else if (a === "--name") name = args[++i];
    else if (a.startsWith("--name=")) name = a.slice("--name=".length);
    else if (!a.startsWith("-") && dir === undefined) dir = a;
  }
  return { dir: dir ?? "", app, workspace, name };
}

export function buildDeployPayload(
  files: readonly CollectedFile[],
): { ok: true; files: DeployFile[] } | { ok: false; error: string } {
  if (files.length === 0) {
    return { ok: false, error: "no files found to deploy" };
  }
  if (files.length > MAX_BUNDLE_FILES) {
    return {
      ok: false,
      error: `too many files: ${files.length} (max ${MAX_BUNDLE_FILES})`,
    };
  }
  const out: DeployFile[] = [];
  let totalBytes = 0;
  for (const f of files) {
    const size = f.bytes.byteLength;
    if (size > MAX_BUNDLE_FILE_BYTES) {
      return {
        ok: false,
        error: `file too large: ${f.path} (${size} bytes, max ${MAX_BUNDLE_FILE_BYTES})`,
      };
    }
    totalBytes += size;
    if (totalBytes > MAX_BUNDLE_TOTAL_BYTES) {
      return {
        ok: false,
        error: `bundle too large: exceeds ${MAX_BUNDLE_TOTAL_BYTES} bytes total`,
      };
    }
    out.push({
      path: f.path,
      content_b64: Buffer.from(f.bytes).toString("base64"),
    });
  }
  return { ok: true, files: out };
}

function rowId(data: unknown): string | undefined {
  if (typeof data !== "object" || data === null) return undefined;
  const r = data as Record<string, unknown>;
  const id = r.id ?? r.resource_id ?? r.resourceId;
  return typeof id === "string" ? id : undefined;
}

function rowUrl(data: unknown): string | undefined {
  if (typeof data !== "object" || data === null) return undefined;
  const r = data as Record<string, unknown>;
  const url = r.url ?? r.origin;
  return typeof url === "string" ? url : undefined;
}

async function resolveExistingApp(
  exec: V1Exec,
  app: string,
  workspace: string | undefined,
): Promise<{ appId: string; url?: string } | null> {
  const got = await exec("browser-app get", { app });
  if (got.ok) {
    const id = rowId(got.data);
    if (id) return { appId: id, url: rowUrl(got.data) };
  }

  const listed = await exec(
    "browser-app list",
    workspace ? { workspace_id: workspace } : {},
  );
  if (listed.ok && Array.isArray(listed.data)) {
    const match = (listed.data as unknown[]).find((row) => {
      const r = row as Record<string, unknown>;
      return r?.name === app || rowId(r) === app;
    });
    const id = match ? rowId(match) : undefined;
    if (id) return { appId: id, url: rowUrl(match) };
  }
  return null;
}

export async function deployBundle(
  opts: DeployOptions,
  files: readonly CollectedFile[],
  exec: V1Exec,
): Promise<DeployResult> {
  const built = buildDeployPayload(files);
  if (!built.ok) return { ok: false, error: built.error };

  let appId: string | undefined;
  let url: string | undefined;
  let created = false;

  if (opts.app) {
    const existing = await resolveExistingApp(exec, opts.app, opts.workspace);
    if (existing) {
      appId = existing.appId;
      url = existing.url;
    }
  }

  if (!appId) {

    const createArgs: Record<string, unknown> = {
      name: opts.name ?? opts.app ?? defaultAppName(opts.dir),
    };
    if (opts.workspace) createArgs.workspace_id = opts.workspace;
    const createdRes = await exec("browser-app create", createArgs);
    if (!createdRes.ok) {
      return {
        ok: false,
        error: createdRes.error ?? "failed to create app",
      };
    }
    appId = rowId(createdRes.data);
    url = rowUrl(createdRes.data) ?? url;
    created = true;
    if (!appId) {
      return { ok: false, error: "create succeeded but returned no app id" };
    }
  }

  const deployed = await exec("browser-app deploy", {
    app: appId,
    files: built.files,
  });
  if (!deployed.ok) {
    return {
      ok: false,
      appId,
      url,
      created,
      error: deployed.error ?? "deploy failed",
    };
  }
  const data = deployed.data as { written?: unknown; id?: unknown } | null;
  const written =
    typeof data?.written === "number" ? data.written : built.files.length;
  const deploymentId = typeof data?.id === "string" ? data.id : undefined;

  return { ok: true, appId, deploymentId, url, created, written };
}

export function defaultAppName(dir: string): string {
  const norm = resolve(dir).split(sep).filter(Boolean);
  const base = norm.at(-1);

  if ((base === "dist" || base === "build") && norm.length >= 2) {
    return norm.at(-2) ?? "My App";
  }
  return base ?? "My App";
}

export function collectDir(dir: string): CollectedFile[] {
  const root = resolve(dir);
  const out: CollectedFile[] = [];
  const walk = (abs: string) => {
    for (const entry of readdirSync(abs, { withFileTypes: true })) {
      const child = join(abs, entry.name);
      if (entry.isDirectory()) {
        walk(child);
      } else if (entry.isFile()) {
        const rel = relative(root, child).split(sep).join("/");
        out.push({ path: rel, bytes: readFileSync(child) });
      }
    }
  };
  walk(root);
  return out;
}

export interface AppDeployCtx {

  readonly out: (s: string) => void;

  readonly print: (s: string) => void;
}

export async function runAppDeploy(
  ctx: AppDeployCtx,
  args: readonly string[],
  exec: V1Exec,
  io: DeployFsIo = { collect: collectDir },
): Promise<number> {
  const opts = parseDeployArgs(args);
  if (!opts.dir) {
    ctx.out(
      "idapt app deploy: missing <dir> — pass the built bundle, e.g. `idapt app deploy ./dist`.",
    );
    return 1;
  }

  let files: CollectedFile[];
  try {
    files = io.collect(opts.dir);
  } catch (err) {
    ctx.out(
      `idapt app deploy: cannot read "${opts.dir}": ${(err as Error).message}`,
    );
    return 1;
  }

  const result = await deployBundle(opts, files, exec);
  if (!result.ok) {
    ctx.out(`idapt app deploy: ${result.error}`);
    return 1;
  }

  ctx.print(
    [
      `Deployed ${result.written} file${result.written === 1 ? "" : "s"} to ${result.appId}${
        result.created ? " (new app)" : ""
      } — now live${result.deploymentId ? ` (deployment ${result.deploymentId})` : ""}.`,
      result.url ? `Open: ${result.url}` : "",
    ]
      .filter(Boolean)
      .join("\n"),
  );
  return 0;
}
