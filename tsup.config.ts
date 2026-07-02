import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "tsup";

const dir = path.dirname(fileURLToPath(import.meta.url));
const sharedDir = path.resolve(dir, "../../shared");
const apiContractsDir = path.resolve(dir, "../api-contracts/src");

const pkgVersion = (
  JSON.parse(readFileSync(path.join(dir, "package.json"), "utf8")) as {
    version: string;
  }
).version;

export default defineConfig({
  entry: { index: "src/index.ts", bin: "src/bin.ts" },
  format: ["esm", "cjs"],
  dts: { entry: { index: "src/index.ts" } },
  sourcemap: true,
  clean: true,
  target: "es2022",
  treeshake: true,
  env: { IDAPT_COMPUTER_VERSION: pkgVersion },
  esbuildOptions(options) {
    options.alias = {
      ...(options.alias ?? {}),
      "@shared": sharedDir,
      "@idapt/api-contracts": apiContractsDir,
    };

    options.sourcesContent = false;
  },
});
