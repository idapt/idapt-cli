
import { spawn } from "node:child_process";
import { VERSION } from "../version";
import { fetchDistTag, isNewer } from "./notifier";

export interface UpgradeIo {
  readonly stdout: (s: string) => void;
  readonly stderr: (s: string) => void;
  readonly env: Record<string, string | undefined>;
}

type Detected =
  | { kind: "run"; name: string; argv: string[]; pkgSpec: string }
  | { kind: "manual"; command: string; reason: string };

export function detectManager(
  scriptPath: string,
  env: Record<string, string | undefined>,
  channel: "latest" | "next",
): Detected {
  const p = (scriptPath || "").replace(/\\/g, "/");
  const tag = channel === "next" ? "@next" : "@latest";
  const spec = `@idapt/cli${tag}`;

  if (p.includes("/_npx/")) {
    return {
      kind: "manual",
      command: `npm install -g ${spec}`,
      reason: "you're running via npx",
    };
  }
  if (
    (process.versions as Record<string, string>).bun ||
    p.includes("/.bun/")
  ) {
    return {
      kind: "run",
      name: "bun",
      argv: ["bun", "add", "-g"],
      pkgSpec: spec,
    };
  }
  const voltaHome = env.VOLTA_HOME?.replace(/\\/g, "/");
  if (voltaHome && p.startsWith(voltaHome)) {

    return {
      kind: "run",
      name: "volta",
      argv: ["volta", "install"],
      pkgSpec: channel === "next" ? "@idapt/cli@next" : "@idapt/cli",
    };
  }
  if (p.includes("/pnpm/") || p.includes("/.pnpm/")) {
    return {
      kind: "run",
      name: "pnpm",
      argv: ["pnpm", "add", "-g"],
      pkgSpec: spec,
    };
  }
  return {
    kind: "run",
    name: "npm",
    argv: ["npm", "install", "-g"],
    pkgSpec: spec,
  };
}

function spawnInherit(cmd: string, args: string[]): Promise<number> {
  return new Promise((resolve) => {

    const child =
      process.platform === "win32"
        ? spawn(process.env.ComSpec || "cmd.exe", ["/c", cmd, ...args], {
            stdio: "inherit",
          })
        : spawn(cmd, args, { stdio: "inherit" });
    child.on("error", () => resolve(127));
    child.on("close", (code) => resolve(code ?? 0));
  });
}

export async function runUpgrade(
  io: UpgradeIo,
  opts: { check?: boolean; next?: boolean },
): Promise<number> {
  const channel: "latest" | "next" = opts.next ? "next" : "latest";
  const latest = await fetchDistTag(channel, 5000);

  if (opts.check) {
    if (!latest) {
      io.stderr("idapt: could not reach the npm registry.\n");
      return 1;
    }
    io.stdout(
      isNewer(latest, VERSION)
        ? `Update available: idapt ${VERSION} → ${latest}. Run \`idapt upgrade\`.\n`
        : `idapt ${VERSION} is up to date (${channel}).\n`,
    );
    return 0;
  }

  if (latest && channel === "latest" && !isNewer(latest, VERSION)) {
    io.stdout(`idapt ${VERSION} is already up to date.\n`);
    return 0;
  }

  const det = detectManager(process.argv[1] ?? "", io.env, channel);
  if (det.kind === "manual") {
    io.stdout(`To update (${det.reason}), run:\n  ${det.command}\n`);
    return 0;
  }

  const args = [...det.argv.slice(1), det.pkgSpec];
  const cmdline = [det.argv[0], ...args].join(" ");
  io.stderr(`Updating idapt via ${det.name}…  (${cmdline})\n`);
  const code = await spawnInherit(det.argv[0], args);
  if (code !== 0) {
    io.stderr(
      `\nAutomatic update failed (exit ${code}). Run it manually:\n  ${cmdline}\n`,
    );
    return code;
  }
  io.stdout("\n✓ Updated. Run `idapt version` to confirm.\n");
  return 0;
}
