
import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import {
  getScaffoldFiles,
  type ScaffoldFile,
  type ScaffoldTemplate,
} from "./templates";

export interface InitOptions {

  readonly name?: string;
  readonly template: ScaffoldTemplate;

  readonly dir: string;

  readonly force?: boolean;
}

export interface InitIo {

  readonly dirIsNonEmpty: (path: string) => boolean;

  readonly writeFile: (path: string, content: string) => void;
}

export interface InitResult {
  readonly ok: boolean;

  readonly written: readonly string[];

  readonly error?: string;
}

export function parseInitArgs(args: readonly string[]): InitOptions {
  let name: string | undefined;
  let template: ScaffoldTemplate = "react";
  let dir = ".";
  let force = false;

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--template" || a === "-t") {
      template = normTemplate(args[++i]);
    } else if (a.startsWith("--template=")) {
      template = normTemplate(a.slice("--template=".length));
    } else if (a === "--dir" || a === "-d") {
      dir = args[++i] ?? ".";
    } else if (a.startsWith("--dir=")) {
      dir = a.slice("--dir=".length) || ".";
    } else if (a === "--force" || a === "-f") {
      force = true;
    } else if (!a.startsWith("-") && name === undefined) {
      name = a;
    }
  }
  return { name, template, dir, force };
}

function normTemplate(v: string | undefined): ScaffoldTemplate {
  return v === "vanilla" ? "vanilla" : "react";
}

export function planInit(opts: InitOptions, io: InitIo): InitResult {
  if (opts.template !== "react" && opts.template !== "vanilla") {
    return {
      ok: false,
      written: [],
      error: `unknown template "${opts.template}" (use react or vanilla)`,
    };
  }
  if (!opts.force && io.dirIsNonEmpty(opts.dir)) {
    return {
      ok: false,
      written: [],
      error: `target "${opts.dir}" is not empty — pass --force to scaffold into it anyway`,
    };
  }

  const files: ScaffoldFile[] = getScaffoldFiles(opts.template, {
    name: opts.name ?? "My App",
  });
  const written: string[] = [];
  for (const f of files) {
    io.writeFile(join(opts.dir, f.path), f.content);
    written.push(f.path);
  }
  return { ok: true, written };
}

export interface AppInitCtx {

  readonly out: (s: string) => void;

  readonly print: (s: string) => void;
}

export function runAppInit(ctx: AppInitCtx, args: readonly string[]): number {
  const opts = parseInitArgs(args);
  const io: InitIo = {
    dirIsNonEmpty: (p) => {
      const abs = resolve(p);
      if (!existsSync(abs)) return false;
      try {
        return readdirSync(abs).length > 0;
      } catch {
        return false;
      }
    },
    writeFile: (p, content) => {
      const abs = resolve(p);
      mkdirSync(dirname(abs), { recursive: true });
      writeFileSync(abs, content, "utf-8");
    },
  };

  const result = planInit(opts, io);
  if (!result.ok) {
    ctx.out(`idapt app init: ${result.error}`);
    return 1;
  }

  ctx.print(
    `Scaffolded a ${opts.template} browser-app in ${opts.dir} (${result.written.length} files).`,
  );
  ctx.out(
    [
      "",
      "Next steps:",
      `  cd ${opts.dir}`,
      "  npm install && npm run build",
      "  idapt app deploy ./dist",
    ].join("\n"),
  );
  return 0;
}

/* c8 ignore start — convenience for callers that already hold file bytes. */

export function readScaffoldFile(dir: string, relPath: string): string {
  return readFileSync(resolve(join(dir, relPath)), "utf-8");
}
/* c8 ignore stop */
