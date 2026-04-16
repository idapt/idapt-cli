---
name: idapt-cli
description: Use the idapt CLI to manage projects, agents, files, chats, tasks, and 200+ AI models from the terminal.
icon: terminal
version: "1.0"
license: MIT
---

# idapt CLI

Command-line tool to manage projects, agents, files, chats, tasks, and 200+ AI models from your terminal.

## Install

```bash
curl -fsSL https://idapt.ai/cli/install | bash
```

## Authentication

```bash
idapt auth login
```

Or use an API key:

```bash
idapt --api-key uk_your_key_here <command>
# Or set it globally:
idapt config set api_key uk_your_key_here
```

## Commands

### Resources

| Command | Actions | Description |
|---------|---------|-------------|
| project | list, create, get, edit, delete, fork, member | Project management |
| agent | list, create, get, edit, delete | Agent management |
| chat | list, create, get, edit, delete, send, messages, export, stop | Chat & messaging |
| file | list, read, write, create, edit, delete, rename, move, mkdir, grep, glob, search, upload, download | File operations |
| task | list, create, get, edit, delete, comment + labels | Task management |
| secret | list, create, get, edit, delete, generate, mount, unmount | Secret management |
| trigger | list, get, create, edit, delete, fire, rotate-secret, runs | Cron + webhook automations |
| notification | list, get, read, send, archive, unarchive, delete, preferences, config | Inbox + project broadcasts |

### Execution

| Command | Actions | Description |
|---------|---------|-------------|
| exec | code, bash | Cloud code & bash runs |
| web | search, fetch | Web search & fetch |
| media | generate, transcribe | Image gen & audio |
| multi-agent | send, list, read | Multi-agent orchestration |

### Discovery & Account

| Command | Actions | Description |
|---------|---------|-------------|
| model | list, search, favorite | Model browsing |
| share | list, create, delete | Resource sharing |
| auth | login, logout, status | Authentication |
| config | set, get, list | CLI configuration |
| api-key | list, create, delete | API key management |

## Global Flags

```
--api-key    API key for authentication
--api-url    Custom API URL
--project    Project slug (default: personal)
-o, --output Output format: table, json, jsonl, quiet
--verbose    Verbose output
--confirm    Skip confirmation prompts
--no-color   Disable color output
```

## Quick Examples

```bash
# List agents
idapt agent list

# Create a chat and send a message
idapt chat create --title "Research"
idapt chat send CHAT_ID "What is quantum computing?"

# Upload a file
idapt file upload report.pdf --project my-project

# Run code in the cloud
idapt exec code --lang python -c "print('Hello from the cloud')"

# Search across all files
idapt file search "authentication"

# List available models
idapt model list

# Export chat to JSON
idapt chat export CHAT_ID -o json

# Notify project members when a build finishes
idapt notification send --project my-project --target all_members \
  --title "Deploy complete" --message "Staging is up" --channels in_app,web_push

# Read your unread inbox
idapt notification list --unread

# Toggle quiet hours 22:00-07:00 Europe/Paris
idapt notification config set --quiet-hours --quiet-start 22:00 \
  --quiet-end 07:00 --timezone Europe/Paris
```

## Output Formats

- `table` — Human-readable (default for TTY)
- `json` — Single JSON object/array
- `jsonl` — One JSON object per line (for piping)
- `quiet` — IDs only (for scripting)

```bash
# Pipe-friendly scripting
idapt agent list -o jsonl | jq '.name'
CHAT_ID=$(idapt chat create --title "Test" -o quiet)
idapt chat send $CHAT_ID "Hello"
```

## Links

- GitHub: https://github.com/idapt/idapt-cli
- CLI overview: https://idapt.ai/help/cli-overview
- CLI authentication: https://idapt.ai/help/cli-authentication
- CLI commands reference: https://idapt.ai/help/cli-commands
- Create API key: https://idapt.ai/#settings
- Developers hub: https://idapt.ai/developers
