

import readline from "node:readline";

export type SelectOption = {
  label: string;
  hint?: string;
};

export type ListState = {
  cursor: number;
  checked: boolean[];
};

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

function renderList(
  stderr: (s: string) => void,
  prompt: string,
  options: SelectOption[],
  state: ListState,
  multi: boolean,
): void {
  const lines: string[] = [prompt];
  options.forEach((option, index) => {
    const marker = multi
      ? state.checked[index]
        ? "[x]"
        : "[ ]"
      : index === state.cursor
        ? ">"
        : " ";
    const pointer = index === state.cursor ? ">" : " ";
    lines.push(
      `  ${pointer} ${marker} ${option.label}${option.hint ? `  (${option.hint})` : ""}`,
    );
  });
  if (multi) {
    lines.push(
      "  ↑/↓ move · space toggle · a all/none · enter confirm · esc cancel",
    );
  } else {
    lines.push("  ↑/↓ move · enter select · esc cancel");
  }
  stderr(`\n${lines.join("\n")}\n`);
}

function eraseLines(stderr: (s: string) => void, count: number): void {
  stderr(`\x1b[${count}A\x1b[J`);
}

export async function selectOne(
  io: {
    isTty: boolean;
    stdout: (s: string) => void;
    stderr: (s: string) => void;
    env: Record<string, string | undefined>;
  },
  prompt: string,
  options: SelectOption[],
): Promise<number | null> {
  if (options.length === 0) return null;
  if (options.length === 1) return 0;
  if (!io.isTty) return null;
  const key = readKey({ isTty: io.isTty, stderr: io.stderr, env: io.env });
  let state: ListState = { cursor: 0, checked: options.map(() => false) };
  renderList(io.stderr, prompt, options, state, false);
  try {
    for (;;) {
      const input = await key.listen();
      if (input === null) return null;
      if (input === "up") state = moveCursor(state, -1);
      else if (input === "down") state = moveCursor(state, 1);
      else if (input === "enter") return state.cursor;
      else if (input === "escape" || input === "q") return null;
      eraseLines(io.stderr, options.length + 3);
      renderList(io.stderr, prompt, options, state, false);
    }
  } finally {
    key.stop();
  }
}

export async function selectMany(
  io: {
    isTty: boolean;
    stdout: (s: string) => void;
    stderr: (s: string) => void;
    env: Record<string, string | undefined>;
  },
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
  renderList(io.stderr, prompt, options, state, true);
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
      eraseLines(io.stderr, options.length + 4);
      renderList(io.stderr, prompt, options, state, true);
    }
  } finally {
    key.stop();
  }
}
