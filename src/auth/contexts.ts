

import {
  type Credentials,
  credentialsPath,
  loadCredentials,
  saveCredentials,
} from "./credentials";

export const DEFAULT_CONTEXT = "default";

export interface AuthContext extends Credentials {

  apiUrl?: string;
}

export interface ContextStore {

  current: string;
  contexts: Record<string, AuthContext>;
}

function isContextStore(value: unknown): value is ContextStore {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as ContextStore).contexts === "object" &&
    (value as ContextStore).contexts !== null
  );
}

export function loadContexts(path = credentialsPath()): ContextStore {
  const raw = loadCredentials(path) as unknown;
  if (isContextStore(raw)) {
    return {
      current: raw.current || DEFAULT_CONTEXT,
      contexts: raw.contexts,
    };
  }
  const legacy = raw as Credentials;

  if (!legacy || Object.keys(legacy).length === 0) {
    return { current: DEFAULT_CONTEXT, contexts: {} };
  }
  return { current: DEFAULT_CONTEXT, contexts: { [DEFAULT_CONTEXT]: legacy } };
}

export function saveContexts(store: ContextStore, path = credentialsPath()) {
  saveCredentials(store as unknown as Credentials, path);
}

export function resolveContextName(
  store: ContextStore,
  opts: { flag?: string; env?: Record<string, string | undefined> },
): string {
  return (
    opts.flag || opts.env?.IDAPT_CONTEXT || store.current || DEFAULT_CONTEXT
  );
}

export function activeContext(store: ContextStore, name: string): AuthContext {
  return store.contexts[name] ?? {};
}

export function putContext(
  store: ContextStore,
  name: string,
  ctx: AuthContext,
): ContextStore {
  return {
    current: name,
    contexts: { ...store.contexts, [name]: ctx },
  };
}

export function removeContext(store: ContextStore, name: string): ContextStore {
  const contexts = { ...store.contexts };
  delete contexts[name];
  const current =
    store.current === name
      ? (Object.keys(contexts)[0] ?? DEFAULT_CONTEXT)
      : store.current;
  return { current, contexts };
}

export function renameContext(
  store: ContextStore,
  from: string,
  to: string,
): ContextStore {
  const ctx = store.contexts[from];
  if (!ctx) return store;
  const contexts = { ...store.contexts };
  delete contexts[from];
  contexts[to] = ctx;
  return { current: store.current === from ? to : store.current, contexts };
}

export function loadActiveCredentials(opts?: {
  flag?: string;
  env?: Record<string, string | undefined>;
}): AuthContext {
  const store = loadContexts();
  return activeContext(store, resolveContextName(store, opts ?? {}));
}

export function saveActiveCredentials(
  creds: AuthContext,
  opts?: { flag?: string; env?: Record<string, string | undefined> },
): void {
  const store = loadContexts();
  const name = resolveContextName(store, opts ?? {});
  saveContexts({
    current: name,
    contexts: { ...store.contexts, [name]: creds },
  });
}

export function clearActiveCredentials(opts?: {
  flag?: string;
  env?: Record<string, string | undefined>;
}): void {
  const store = loadContexts();
  const name = resolveContextName(store, opts ?? {});
  saveContexts(removeContext(store, name));
}

export interface ContextIo {
  readonly print: (s: string) => void;
  readonly err: (s: string) => void;
}

export function runContextCommand(
  sub: "list" | "switch" | "rename" | "remove",
  args: readonly string[],
  io: ContextIo,
): number {
  const store = loadContexts();
  const names = Object.keys(store.contexts);

  if (sub === "list") {
    if (names.length === 0) {
      io.print("No accounts. Run `idapt login`.\n");
      return 0;
    }
    for (const name of names.sort()) {
      const marker = name === store.current ? "*" : " ";
      const ctx = store.contexts[name] ?? {};
      const where = ctx.apiUrl ? ` (${ctx.apiUrl})` : "";
      const ws = ctx.defaultWorkspaceSlug
        ? ` [workspace: ${ctx.defaultWorkspaceSlug}]`
        : "";
      io.print(`${marker} ${name}${where}${ws}\n`);
    }
    return 0;
  }

  if (sub === "switch") {
    const name = args[0];
    if (!name) {
      io.err("idapt auth switch <name>\n");
      return 1;
    }
    if (!store.contexts[name]) {
      io.err(
        `No account named "${name}". Known: ${names.join(", ") || "(none)"}\n`,
      );
      return 1;
    }
    saveContexts({ ...store, current: name });
    io.print(`Switched to "${name}".\n`);
    return 0;
  }

  if (sub === "rename") {
    const [from, to] = args;
    if (!from || !to) {
      io.err("idapt auth rename <from> <to>\n");
      return 1;
    }
    if (!store.contexts[from]) {
      io.err(`No account named "${from}".\n`);
      return 1;
    }
    if (store.contexts[to]) {
      io.err(`An account named "${to}" already exists.\n`);
      return 1;
    }
    saveContexts(renameContext(store, from, to));
    io.print(`Renamed "${from}" to "${to}".\n`);
    return 0;
  }

  const name = args[0];
  if (!name) {
    io.err("idapt auth remove <name>\n");
    return 1;
  }
  if (!store.contexts[name]) {
    io.err(`No account named "${name}".\n`);
    return 1;
  }
  const next = removeContext(store, name);
  saveContexts(next);
  io.print(
    Object.keys(next.contexts).length > 0
      ? `Removed "${name}". Now using "${next.current}".\n`
      : `Removed "${name}". No accounts left - run \`idapt login\`.\n`,
  );
  return 0;
}
