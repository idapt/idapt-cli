

import { execFile as execFileCb } from "node:child_process";
import { existsSync, mkdirSync, rmSync, statSync } from "node:fs";
import { join } from "node:path";
import { promisify } from "node:util";

const execFile = promisify(execFileCb);

const CLI_DIR = join(__dirname, "..");
const SDK_DIR = join(CLI_DIR, "..", "sdk");
const LOCK = join(CLI_DIR, "dist", ".build-lock");

const ARTIFACTS = [
  join(CLI_DIR, "dist", "bin.js"),
  join(CLI_DIR, "dist", "index.js"),
];

const SOURCES = [
  join(CLI_DIR, "src", "bin.ts"),
  join(CLI_DIR, "src", "index.ts"),

  join(SDK_DIR, "src", "index.ts"),
];

function isStale(): boolean {
  for (const artifact of ARTIFACTS) {
    if (!existsSync(artifact)) return true;
    const built = statSync(artifact).mtimeMs;
    for (const source of SOURCES) {
      if (existsSync(source) && statSync(source).mtimeMs > built) return true;
    }
  }
  return false;
}

const sleep = (ms: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));

export async function ensureCliBuilt(): Promise<void> {
  if (!isStale()) return;

  mkdirSync(join(CLI_DIR, "dist"), { recursive: true });
  let holdsLock = false;
  try {
    mkdirSync(LOCK);
    holdsLock = true;
  } catch {

  }

  if (!holdsLock) {
    for (let waited = 0; waited < 300_000; waited += 250) {
      await sleep(250);
      if (!existsSync(LOCK) && !isStale()) return;
    }
    throw new Error("timed out waiting for another worker's CLI build");
  }

  try {
    await execFile("npm", ["run", "build"], { cwd: CLI_DIR });
  } finally {
    rmSync(LOCK, { recursive: true, force: true });
  }
}
