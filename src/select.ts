

import readline from "node:readline";

export type SelectOption = {
  label: string;
  hint?: string;
};

export type ListState = {
  cursor: number;
  checked: boolean[];
};

const VIEWPORT_ROWS = 20;

export function moveCursor(state: ListState, count: number): ListState {
  const cursor = Math.max(
    0,
    Math.min(state.checked.length - 1, state.cursor + count),
  );
  return cursor === state.cursor ? state : { ...state, cursor };
}

export function toggleAt(state: ListState, index?: number): ListState {
  const at = index ?? state.cursor;
  if (at < 0 || at >= state.checked.length) return state;
  const checked = state.checked.slice();
  checked[at] = !checked[at];
  return { ...state, checked };
}

export function setAll(state: ListState, value: boolean): ListState {
  return { ...state, checked: state.checked.map(() => value) };
}

export function viewportStart(
  cursor: number,
  total: number,
  size = VIEWPORT_ROWS,
): number {
  if (total <= size) return 0;
  const half = Math.floor(size / 2);
  return Math.max(0, Math.min(total - size, cursor - half));
}

export function sanitizeLabel(label: string, columns: number): string {
  const flat = label.replace(/\s+/g, " ").trim();

  const max = Math.max(20, columns - 28);
  return flat.length > max ? `${flat.slice(0, max - 1)}…` : flat;
}

function columns(): number {
  return process.stdout.columns || 100;
}

function painted(checked: boolean, cursor: boolean, flat: boolean): string {
  if (flat) return checked ? "\x1b[32m[x]\x1b[0m" : "[ ]";
  if (cursor) return "\x1b[36m>\x1b[0m";
  return " ";
}

type KeyInput = "up" | "down" | "space" | "enter" | "escape" | "a" | "A" | "q";

function readKey(io: {
  isTty: boolean;
  stderr: (s: string) => void;
  prompt?: (q: string) => Promise<string>;
  env: Record<string, string | undefined>;
}): {
  listen(): Promise<KeyInput | null>;
  stop(): void;
} {
  const stdin = process.stdin;
  if (!io.isTty || !stdin.isTTY) {
    return { listen: async () => null, stop: () => {} };
  }
  const rl = readline.createInterface({ input: stdin, escapeCodeTimeout: 50 });
  readline.emitKeypressEvents(stdin, rl);
  const wasRaw = stdin.isRaw;
  stdin.setRawMode(true);
  const waiters: Array<(key: KeyInput | null) => void> = [];

  const onKeypress = (
    _: unknown,
    key: { name?: string; ctrl?: boolean; sequence?: string } | undefined,
  ): void => {
    if (!key) return;
    if (key.ctrl && key.name === "c") {
      for (const w of waiters) w("escape");
      return;
    }
    let mapped: KeyInput | null = null;
    if (key.name === "up" || key.sequence === "k") mapped = "up";
    else if (key.name === "down" || key.sequence === "j") mapped = "down";
    else if (key.name === "space") mapped = "space";
    else if (key.name === "return" || key.name === "enter") mapped = "enter";
    else if (key.name === "escape") mapped = "escape";
    else if (key.name === "a") mapped = "a";
    else if (key.name === "q") mapped = "q";
    if (mapped) for (const w of waiters.splice(0)) w(mapped);
  };

  stdin.on("keypress", onKeypress);
  return {
    listen: () =>
      new Promise<KeyInput | null>((resolve) => waiters.push(resolve)),
    stop: () => {
      stdin.off("keypress", onKeypress);
      stdin.setRawMode(wasRaw);
      rl.close();
    },
  };
}

type FrameLayout = {
  rows: string[];

  height: number;
};

function renderFrame(
  prompt: string,
  options: SelectOption[],
  state: ListState,
  multi: boolean,
): FrameLayout {
  const width = columns();
  const start = viewportStart(state.cursor, options.length);
  const end = Math.min(options.length, start + VIEWPORT_ROWS);
  const rows: string[] = [`\x1b[1m${prompt}\x1b[0m`];
  for (let index = start; index < end; index++) {
    const option = options[index];
    const cursor = index === state.cursor;
    const marker = multi
      ? painted(state.checked[index], false, true)
      : painted(false, cursor, false);
    const label = sanitizeLabel(option.label, width);
    const hint = option.hint
      ? `  (\x1b[2m${sanitizeLabel(option.hint, width)}\x1b[0m)`
      : "";
    const line = `  ${cursor ? "\x1b[36m>\x1b[0m" : " "} ${marker} ${cursor ? `\x1b[1m${label}\x1b[0m` : label}${hint}`;
    rows.push(line);
  }
  const footer = multi
    ? "  ↑/↓ move · space toggle · a all/none · enter confirm · esc cancel"
    : "  ↑/↓ move · enter select · esc cancel";
  const checkedCount = multi
    ? `  · \x1b[2m${state.checked.filter(Boolean).length} selected\x1b[0m`
    : "";
  const position = `\x1b[2m${state.cursor + 1}/${options.length}\x1b[0m`;
  rows.push(`  ${position}${checkedCount}${footer}`);

  return { rows, height: rows.length + 1 };
}

function paint(stderr: (s: string) => void, frame: FrameLayout): void {
  stderr(`\n${frame.rows.join("\n")}\n`);
}

function erase(stderr: (s: string) => void, frame: FrameLayout): void {
  stderr(`\x1b[${frame.height}A\x1b[J`);
}

type PickerIo = {
  isTty: boolean;
  stdout: (s: string) => void;
  stderr: (s: string) => void;
  env: Record<string, string | undefined>;
};

export async function selectOne(
  io: PickerIo,
  prompt: string,
  options: SelectOption[],
): Promise<number | null> {
  if (options.length === 0) return null;
  if (options.length === 1) return 0;
  if (!io.isTty) return null;
  const key = readKey({ isTty: io.isTty, stderr: io.stderr, env: io.env });
  let state: ListState = { cursor: 0, checked: options.map(() => false) };
  let frame = renderFrame(prompt, options, state, false);
  paint(io.stderr, frame);
  try {
    for (;;) {
      const input = await key.listen();
      if (input === null) return null;
      if (input === "up") state = moveCursor(state, -1);
      else if (input === "down") state = moveCursor(state, 1);
      else if (input === "enter") return state.cursor;
      else if (input === "escape" || input === "q") return null;
      erase(io.stderr, frame);
      frame = renderFrame(prompt, options, state, false);
      paint(io.stderr, frame);
    }
  } finally {

    erase(io.stderr, frame);
    key.stop();
  }
}

export async function selectMany(
  io: PickerIo,
  prompt: string,
  options: SelectOption[],
  defaultChecked = true,
): Promise<number[] | null> {
  if (options.length === 0) return [];
  if (!io.isTty) return null;
  const key = readKey({ isTty: io.isTty, stderr: io.stderr, env: io.env });
  let state: ListState = {
    cursor: 0,
    checked: options.map(() => defaultChecked),
  };
  let frame = renderFrame(prompt, options, state, true);
  paint(io.stderr, frame);
  try {
    for (;;) {
      const input = await key.listen();
      if (input === null) return null;
      if (input === "up") state = moveCursor(state, -1);
      else if (input === "down") state = moveCursor(state, 1);
      else if (input === "space") state = toggleAt(state);
      else if (input === "a" || input === "A")
        state = setAll(state, !state.checked.every(Boolean));
      else if (input === "enter") {
        const selected: number[] = [];
        state.checked.forEach((isChecked, index) => {
          if (isChecked) selected.push(index);
        });
        return selected;
      } else if (input === "escape" || input === "q") return null;
      erase(io.stderr, frame);
      frame = renderFrame(prompt, options, state, true);
      paint(io.stderr, frame);
    }
  } finally {
    erase(io.stderr, frame);
    key.stop();
  }
}
