

import type { V1CommandSpec } from "./catalog";
import type { RenderMode } from "./format";

const FRAMES = ["-", "\\", "|", "/"];

const SHOW_AFTER_MS = 400;
const FRAME_MS = 120;

export interface ProgressIo {
  readonly spec: V1CommandSpec;
  readonly isTty: boolean;
  readonly mode: RenderMode;
  readonly err: (s: string) => void;
  readonly quiet?: boolean;

  readonly label?: string;
}

export function shouldShowProgress(io: ProgressIo): boolean {
  if (!io.isTty || io.quiet) return false;
  if (io.mode !== "table") return false;

  if (io.label) return true;

  return io.spec.async === true;
}

export async function withProgress<T>(
  io: ProgressIo,
  work: (signal: AbortSignal) => Promise<T>,
): Promise<T> {
  const controller = new AbortController();
  const onSigint = () => controller.abort();
  process.once("SIGINT", onSigint);

  let frame = 0;
  let ticker: NodeJS.Timeout | undefined;
  let started = false;
  const label = io.label ?? `${io.spec.command}`;

  const startTimer = shouldShowProgress(io)
    ? setTimeout(() => {
        started = true;
        ticker = setInterval(() => {
          frame = (frame + 1) % FRAMES.length;
          io.err(`\r${FRAMES[frame]} ${label}...`);
        }, FRAME_MS);

        ticker.unref?.();
      }, SHOW_AFTER_MS)
    : undefined;
  startTimer?.unref?.();

  try {
    return await work(controller.signal);
  } finally {
    if (startTimer) clearTimeout(startTimer);
    if (ticker) clearInterval(ticker);

    if (started) io.err(`\r${" ".repeat(label.length + 6)}\r`);
    process.removeListener("SIGINT", onSigint);
  }
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`;
}

