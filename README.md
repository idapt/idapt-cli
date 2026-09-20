# @idapt/cli

The **idapt CLI** — drive your [idapt](https://idapt.app) AI workspace from the
terminal or any script: agents, chats, files (Drive), 200+ models, cloud
computers, secrets, triggers, sharing, and more — all over the public v1 API.

It's also the **library** that powers the `idapt` command, so you can embed the
exact same `idapt <resource> <verb>` grammar in your own tools.

## Install

```bash
npm install -g @idapt/cli      # provides the `idapt` command
# or run without installing:
npx @idapt/cli idapt agent list
```

## Quick start

```bash
idapt login                       # sign in in your browser
idapt                             # what can this thing do?

idapt agent list                  # your agents
idapt chat create --title "Hello" # start a chat
idapt drive list                  # browse your Drive
idapt models list                 # 200+ available models
```

For CI, use an API key instead of the browser flow:

```bash
export IDAPT_API_KEY=uk_...       # or: idapt login --api-key-stdin
```

Pick the workspace commands run in, once:

```bash
idapt workspace use acme/api      # or -w acme/api per command
```

Output is a human table on a TTY and JSON when piped; force it with
`-o json|jsonl|quiet`. Every command self-documents, and every form works:

```bash
idapt help                     # the index
idapt <resource>               # a resource's verbs
idapt help <resource> <verb>   # arguments, types, defaults
idapt <resource> <verb> --help # the same contract
idapt instructions <resource>  # when/why playbook
```

Lists page: a truncated result prints its next cursor, and `--all` walks every
page. Exit codes are structured, so a script can tell an expired credential (2)
from a missing resource (4) from a rate limit (7).

## Use as a library

```ts
import { execute, createFetchTransport } from "@idapt/cli";

const transport = createFetchTransport({
  baseUrl: "https://idapt.app",
  token: process.env.IDAPT_API_KEY,
});

const r = await execute("idapt drive list --parent-id folder_x", {
  transport,
  mode: "table", // "json" | "jsonl" | "quiet" | "table" | "llm"
});
console.log(r.ok ? r.rendered : r.error);
```

For a fully typed, method-per-endpoint client, see
[`@idapt/sdk`](https://www.npmjs.com/package/@idapt/sdk).

## Links

- [CLI overview & docs](https://idapt.app/help/cli-overview)
- [Command reference](https://idapt.app/help/cli-commands)
- [Public API reference](https://idapt.app/api/v1/docs)

## License

Apache-2.0 — see [LICENSE](./LICENSE), [NOTICE](./NOTICE), and
[THIRD_PARTY_NOTICES.txt](./THIRD_PARTY_NOTICES.txt).
