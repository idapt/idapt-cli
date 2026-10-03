

import { createRequire } from "node:module";

let suppressed = false;

function suppressSqliteExperimentalWarning(): void {
  if (suppressed) return;
  suppressed = true;
  const originalEmitWarning = process.emitWarning.bind(process);
  process.emitWarning = (warning: string | Error, ...args: unknown[]): void => {
    const message = typeof warning === "string" ? warning : warning?.message;
    if (message && /experimental/i.test(message) && /\bsqlite\b/i.test(message))
      return;
    (originalEmitWarning as (...a: unknown[]) => void)(warning, ...args);
  };
}

suppressSqliteExperimentalWarning();

export function getSqliteDatabaseSync(): typeof import("node:sqlite").DatabaseSync {
  const { DatabaseSync } = createRequire(import.meta.url)(
    "node:sqlite",
  ) as typeof import("node:sqlite");
  return DatabaseSync;
}
