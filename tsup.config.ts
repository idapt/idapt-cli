import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "tsup";

const dir = path.dirname(fileURLToPath(import.meta.url));
const sharedDir = path.resolve(dir, "../../shared");
const apiContractsDir = path.resolve(dir, "../api-contracts/src");

const sdkDir = path.resolve(dir, "../sdk/src");
const pkgVersion = (
  JSON.parse(readFileSync(path.join(dir, "package.json"), "utf8")) as {
    version: string;
  }
).version;

function gitCommit(): string {
  try {
    return execFileSync("git", ["rev-parse", "--short=12", "HEAD"], {
      cwd: dir,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return "unknown";
  }
}

export default defineConfig({
  entry: { index: "src/index.ts", bin: "src/bin.ts" },
  format: ["esm", "cjs"],
  dts: { entry: { index: "src/index.ts" } },
  sourcemap: true,
  clean: true,
  target: "es2022",
  treeshake: true,
  env: {
    IDAPT_CLI_VERSION: pkgVersion,
    IDAPT_CLI_COMMIT: gitCommit(),
    IDAPT_CLI_BUILT_AT: new Date().toISOString(),
  },
  esbuildOptions(options) {
    options.alias = {
      ...(options.alias ?? {}),
      "@shared": sharedDir,
      "@idapt/api-contracts": apiContractsDir,
      "@idapt/sdk": sdkDir,
    };

    options.sourcesContent = false;
  },
});
