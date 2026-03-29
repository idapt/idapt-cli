# idapt CLI

The official command-line tool for [idapt](https://idapt.ai).

## Install

```bash
curl -fsSL https://idapt.ai/cli/install | bash
```

Or download directly from [GitHub Releases](https://github.com/idapt/idapt-cli/releases).

## Usage

### User-Facing CLI

Interact with idapt from any terminal — manage projects, agents, files, chats, machines, and more.

```bash
# Authenticate
idapt auth login --api-key idapt_...
idapt auth status

# Manage resources
idapt project list -o json
idapt agent create --name "My Agent" --system-prompt "You are helpful"
idapt file upload ./data.csv
idapt chat send my-chat --message "Hello"
idapt machine exec prod-server "ls -la"

# JSON input for agents/automation
echo '{"name":"agent","icon":"emoji/🤖"}' | idapt agent create --json -
```

### Per-Machine Daemon

On managed machines, the CLI runs as a daemon providing TLS, auth, and proxying:

```bash
idapt serve --config /etc/idapt/config.json
```

## Command Groups

| Group | Commands | Description |
|-------|----------|-------------|
| `auth` | login, logout, status | Authentication |
| `config` | set, get, list | CLI configuration (~/.idapt/) |
| `project` | list, create, get, edit, delete, fork, member | Project management |
| `agent` | list, create, get, edit, delete | Agent management |
| `chat` | list, create, get, edit, delete, send, messages, export, stop | Chat & messaging |
| `file` | list, read, write, create, edit, delete, rename, move, mkdir, grep, glob, search, upload, download | File operations |
| `kb` | list, create, get, edit, delete, ask, search, ingest + note subcommands | Knowledge bases |
| `task` | list, create, get, edit, delete, comment + label subcommands | Task management |
| `machine` | list, create, get, edit, start, stop, terminate + exec, tmux, file, firewall, user, port | Machine management |
| `script` | list, create, get, edit, delete, run, run-sequence, pin, unpin, runs | Script management |
| `secret` | list, create, get, edit, delete, generate, mount, unmount | Secret management |
| `store` | search + skill/kb/script/agent install | Marketplace |
| `model` | list, search, favorite | Model browsing |
| `exec` | code, bash | Sandboxed code execution |
| `web` | search, fetch | Web search & fetch |
| `image` | generate | Image generation |
| `settings` | get, set | Account settings |
| `profile` | get, edit | Profile management |
| `api-key` | list, create, delete | API key management |
| `share` | list, create, delete | Resource sharing |
| `serve` | (daemon) | Per-machine daemon |
| `firewall` | list, add, remove | Local firewall (daemon) |
| `version` | | Print CLI version |
| `update` | | Self-update binary |

## Global Flags

```
--api-key string   API key for authentication (or IDAPT_API_KEY env)
--api-url string   API base URL (default https://idapt.ai)
--project string   Default project slug (or IDAPT_PROJECT env)
-o, --output       Output format: table|json|jsonl|quiet
--verbose          Show request/response details
--confirm          Skip confirmation prompts for destructive ops
--no-color         Disable color output
```

## Input/Output

**Input modes** (for create/edit commands):
- Named flags: `--name "My Agent" --icon "emoji/🤖"`
- JSON flag: `--json '{"name":"test","systemPrompt":"..."}'`
- JSON from stdin: `echo '{}' | idapt agent create --json -`
- File flags: `--system-prompt-file ./prompt.md`

**Output formats** (`-o` flag):
- `table` — human-readable columns (default for TTY)
- `json` — machine-readable JSON (default when piped)
- `jsonl` — one JSON object per line
- `quiet` — IDs only

## Architecture

```
services/idapt/
├── cmd/                    # Cobra command files (one per resource group)
│   ├── root.go             # Global flags, PersistentPreRunE, command wiring
│   ├── auth.go             # auth login/logout/status
│   ├── agent.go            # agent CRUD
│   ├── machine.go          # machine wiring
│   ├── machine_core.go     # machine lifecycle
│   ├── machine_file.go     # machine remote files
│   ├── machine_tmux.go     # machine tmux sessions
│   └── ...                 # 24 resource groups total
├── internal/
│   ├── api/                # REST API HTTP client (auth, retry, SSE, upload/download)
│   ├── cliconfig/          # CLI config (~/.idapt/config.json)
│   ├── credential/         # Credential storage (~/.idapt/credentials.json)
│   ├── output/             # Output formatters (table, JSON, JSONL, quiet)
│   ├── input/              # --json and --*-file flag parsing
│   ├── resolve/            # Resource name → ID resolution with caching
│   ├── cmdutil/            # Factory (DI), global flags, exit codes, confirm
│   ├── httpclient/         # Version header transport (User-Agent, X-Idapt-Version)
│   ├── auth/               # Daemon JWT/HMAC/API key validation
│   ├── config/             # Daemon config (/etc/idapt/config.json)
│   ├── proxy/              # Daemon reverse proxy
│   ├── firewall/           # Daemon iptables management
│   └── ...                 # Other daemon packages
└── tests/integration/      # Integration tests (//go:build integration)
```

## API Version Handling

Every request includes `User-Agent: idapt-cli/{version}` and `X-Idapt-Version: {api-version}` via `internal/httpclient`. The CLI ignores unknown response fields and handles missing optional fields for forward/backward compatibility. See root `CLI.md` and `API_Versioning.md`.

## Testing

```bash
# Unit tests (576 tests, no infrastructure needed)
go test ./...
go test -race ./...

# Integration tests (requires running API server)
npm run test:cli:integration                    # handles infra lifecycle
# or manually:
IDAPT_TEST_BASE_URL=http://localhost:3001 \
  go test -tags=integration -v ./tests/integration/...
```

## License

MIT — see [LICENSE](LICENSE)
