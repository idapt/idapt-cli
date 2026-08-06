
import { executeCommand } from "../execute";
import { createFetchTransport } from "../transport";
import { USER_AGENT } from "../version";
import { type AppDeployCtx, runAppDeploy, type V1Exec } from "./deploy";
import { type AppInitCtx, runAppInit } from "./init";

export {
  buildDeployPayload,
  type CollectedFile,
  collectDir,
  type DeployFile,
  type DeployOptions,
  type DeployResult,
  defaultAppName,
  deployBundle,
  MAX_BUNDLE_FILE_BYTES,
  MAX_BUNDLE_FILES,
  MAX_BUNDLE_TOTAL_BYTES,
  parseDeployArgs,
} from "./deploy";
export {
  type InitIo,
  type InitOptions,
  type InitResult,
  parseInitArgs,
  planInit,
} from "./init";
export {
  getScaffoldFiles,
  type ScaffoldFile,
  type ScaffoldOptions,
  type ScaffoldTemplate,
} from "./templates";

export interface AppCtx extends AppInitCtx, AppDeployCtx {
  readonly baseUrl: string;

  readonly token?: string;
}

export const APP_USAGE = `idapt app - local browser-app developer loop

Usage:
  idapt app init [name] [--template react|vanilla] [--dir .] [--force]
  idapt app deploy <dir> [--app <id|name>] [--workspace <id>] [--name <name>]

  init    Scaffold an npm browser-app project locally (then npm install && npm run build)
  deploy  Upload a locally-built dist/ to a browser-app (creates one if --app is absent/unknown)`;

export async function runApp(
  ctx: AppCtx,
  args: readonly string[],
): Promise<number> {
  const sub = args[0];
  const rest = args.slice(1);

  if (sub === "init") {
    return runAppInit(ctx, rest);
  }
  if (sub === "deploy") {
    if (!ctx.token) {

      ctx.out(
        "idapt app deploy: not signed in. Run `idapt login` (or set IDAPT_API_KEY).",
      );
      return 1;
    }
    const transport = createFetchTransport({
      baseUrl: ctx.baseUrl,
      token: ctx.token,
      userAgent: USER_AGENT,
    });
    const exec: V1Exec = (commandName, cmdArgs) =>
      executeCommand(commandName, cmdArgs, { transport, mode: "json" });
    return runAppDeploy(ctx, rest, exec);
  }

  if (!sub || sub === "--help" || sub === "-h" || sub === "help") {
    ctx.print(APP_USAGE);
    return sub ? 0 : 1;
  }
  ctx.out(`idapt app: unknown subcommand "${sub}".\n\n${APP_USAGE}`);
  return 1;
}
