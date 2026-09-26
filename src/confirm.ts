

import { createInterface } from "node:readline/promises";
import type { V1CommandSpec } from "./catalog";
import { EXIT_ERROR, EXIT_VALIDATION } from "./exit-codes";

export interface ConfirmIo {
  readonly isTty: boolean;
  readonly err: (s: string) => void;

  readonly prompt?: (question: string) => Promise<string>;
}

export interface ConfirmDecision {
  readonly confirmed: boolean;

  readonly message: string;

  readonly code: number;
}

async function defaultPrompt(question: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stderr });
  try {
    return await rl.question(question);
  } finally {
    rl.close();
  }
}

export function describeTarget(
  spec: V1CommandSpec,
  positionals: readonly string[],
): string {
  const target = positionals[0];
  return target ? `${spec.resource} ${target}` : `this ${spec.resource}`;
}

export async function confirmDestructive(
  spec: V1CommandSpec,
  positionals: readonly string[],
  io: ConfirmIo,
): Promise<ConfirmDecision> {
  const what = describeTarget(spec, positionals);

  if (!io.isTty) {
    return {
      confirmed: false,
      message:
        `idapt: \`${spec.command}\` cannot be undone and there is no terminal to confirm on.\n` +
        "  Pass -y/--yes if you really mean to run it unattended.",
      code: EXIT_VALIDATION,
    };
  }

  io.err(`This permanently deletes ${what}. It cannot be undone.\n`);
  const answer = (
    await (io.prompt ?? defaultPrompt)("Type 'yes' to continue: ")
  )
    .trim()
    .toLowerCase();

  if (answer === "yes" || answer === "y") {
    return { confirmed: true, message: "", code: 0 };
  }
  return { confirmed: false, message: "Cancelled.", code: EXIT_ERROR };
}
