# CLI Guide

LlamaSitter ships with a nested CLI for four jobs:

- start the proxy and dashboard
- inspect captured usage data
- manage config safely without hand-editing YAML
- locate the macOS desktop app's managed files

If you are new to the project, read this guide top to bottom once. After that, the generated command reference under [`reference/cli`](reference/cli/llamasitter.md) is the fastest way to look up exact flags.

## Command Map

| Command | What it is for | Use it when |
| --- | --- | --- |
| `llamasitter serve` | Starts the runtime | You want the proxy, API, storage, and UI running |
| `llamasitter doctor` | Verifies config and upstream reachability | You want to confirm setup before or after changes |
| `llamasitter stats` | Shows aggregate usage totals | You want a quick summary of what has been captured |
| `llamasitter tail` | Shows recent captured requests | You want to inspect the latest traffic |
| `llamasitter export` | Dumps stored requests | You want to analyze data elsewhere |
| `llamasitter config ...` | Creates, validates, and edits config | You want to change listeners, storage, privacy, or UI settings |
| `llamasitter desktop ...` | Prints macOS app-managed paths | You want the CLI to target the same files as the desktop app |
| `llamasitter completion ...` | Generates shell completion scripts | You use the CLI often and want tab completion |

## Five-Minute Quick Start

This is the shortest path from a fresh checkout to a working proxy.

### 1. Make sure Ollama is already running

The default LlamaSitter config expects Ollama at `http://127.0.0.1:11434`.

### 2. Create a config file

```bash
llamasitter config init
```

By default this writes `llamasitter.yaml` in the current directory.

If you want to inspect the generated file first:

```bash
llamasitter config init --dry-run
```

### 3. Validate the config before starting anything

```bash
llamasitter config validate --config llamasitter.yaml
llamasitter doctor --config llamasitter.yaml
```

Use `validate` when you only care about YAML structure and values. Use `doctor` when you also want to check storage access and whether the configured upstream Ollama endpoint is reachable.

### 4. Start LlamaSitter

```bash
llamasitter serve --config llamasitter.yaml
```

With the default config:

- the proxy listens on `127.0.0.1:11435`
- the dashboard UI listens on `127.0.0.1:11438`
- the SQLite database is stored at `~/.llamasitter/llamasitter.db`

### 5. Send one request through the proxy

Point your client at `127.0.0.1:11435` instead of Ollama's default `127.0.0.1:11434`.

You can verify the setup with `curl`:

```bash
curl http://127.0.0.1:11435/api/chat \
  -H 'Content-Type: application/json' \
  -H 'X-LlamaSitter-Client-Type: curl' \
  -H 'X-LlamaSitter-Client-Instance: terminal' \
  -H 'X-LlamaSitter-Agent-Name: quickstart' \
  -H 'X-LlamaSitter-Session-Id: hello-session' \
  -d '{
    "model": "llama3",
    "stream": false,
    "messages": [
      {"role": "user", "content": "Reply with exactly five words."}
    ]
  }'
```

The `X-LlamaSitter-*` headers are optional, but they are the easiest way to populate attribution fields while testing.

### 6. Inspect what was captured

```bash
llamasitter stats --config llamasitter.yaml
llamasitter tail --config llamasitter.yaml -n 10
```

Open the dashboard in your browser at `http://127.0.0.1:11438` if UI is enabled.

## How To Think About The CLI

LlamaSitter works best when you keep four concepts separate:

- A `listener` is one local proxy endpoint. It has a bind address, an upstream Ollama URL, and optional default attribution tags.
- `storage` is the SQLite database where request metadata is persisted.
- `privacy` controls how much request and response content is stored and which fields are redacted.
- `ui` controls the embedded dashboard listener.

The CLI mirrors that structure:

- runtime commands live at the top level
- config editing lives under `llamasitter config ...`
- macOS desktop helpers live under `llamasitter desktop ...`

## Config File Basics

### Default path behavior

If you do not pass `--config`, commands use `llamasitter.yaml` in the current directory.

To see the resolved path explicitly:

```bash
llamasitter config path
```

To target another file:

```bash
llamasitter config view --config /tmp/llamasitter.yaml
```

### Safe editing model

Config mutation commands do not require hand-editing YAML. Instead, use commands such as:

- `llamasitter config listener add`
- `llamasitter config listener update`
- `llamasitter config ui set-listen-addr`
- `llamasitter config storage set-sqlite-path`
- `llamasitter config privacy set-persist-bodies`

Most write commands support `--dry-run` so you can preview the resulting YAML before saving it.

Example:

```bash
llamasitter config listener add \
  --config llamasitter.yaml \
  --name openwebui \
  --listen-addr 127.0.0.1:11436 \
  --upstream-url http://127.0.0.1:11434 \
  --tag client_type=openwebui \
  --tag client_instance=docker \
  --dry-run
```

When a mutation is written, the CLI prints a restart hint. The running service does not hot-reload config automatically.

## Runtime Commands

### `llamasitter serve`

Use this to start the actual system. It loads config, opens storage, starts the configured proxy listeners, starts the local API, and serves the dashboard UI if enabled.

Typical usage:

```bash
llamasitter serve --config llamasitter.yaml
```

Use this command when:

- you want to proxy real Ollama traffic
- you want the dashboard available
- you want requests written into the local SQLite store

### `llamasitter doctor`

Use this to confirm that your setup is healthy before you start sending traffic.

It checks:

- the selected config file can be parsed and validated
- the SQLite storage path is usable
- each listener's upstream Ollama `/api/version` endpoint is reachable
- whether the UI is enabled and where it is configured to listen

Examples:

```bash
llamasitter doctor --config llamasitter.yaml
llamasitter doctor --config llamasitter.yaml --output json
```

`doctor` is a good fit for CI checks, shell scripts, and "why is nothing showing up?" debugging.

### `llamasitter stats`

Use this for a compact usage summary from local storage.

The default table output includes:

- total requests
- successful requests
- aborted requests
- active session count
- prompt, output, and total tokens
- average request duration
- a per-model breakdown

Examples:

```bash
llamasitter stats --config llamasitter.yaml
llamasitter stats --config llamasitter.yaml --output json
llamasitter stats --config llamasitter.yaml --output yaml
```

Important detail: JSON and YAML output are richer than the default table. They include the full summary payload, including contributor breakdowns such as:

- `by_client_type`
- `by_client_instance`
- `by_agent_name`

The current CLI does not have a dedicated `sessions` command. `stats` reports the active session count, while the dashboard and local API provide the fuller session view.

### `llamasitter tail`

Use this to inspect the latest captured requests.

The default table shows:

- request start time
- HTTP status
- model
- total tokens
- duration in milliseconds
- client type and instance
- session id

Examples:

```bash
llamasitter tail --config llamasitter.yaml -n 20
llamasitter tail --config llamasitter.yaml --output json
```

Use JSON or YAML when you want the full stored request records for scripting.

### `llamasitter export`

Use this to dump stored request data for offline analysis.

Examples:

```bash
llamasitter export --config llamasitter.yaml --format json
llamasitter export --config llamasitter.yaml --format csv --output requests.csv
```

Formats:

- `json` exports the full request objects
- `csv` exports a compact table with `request_id`, `started_at`, `model`, `http_status`, `total_tokens`, and `request_duration_ms`

## Config Commands

### Inspecting config

Use these commands when you want to understand the current file without editing it:

```bash
llamasitter config path
llamasitter config view
llamasitter config validate
llamasitter config listener list
llamasitter config listener show default
llamasitter config ui show
llamasitter config storage show
llamasitter config privacy show
```

`view` is the best general-purpose overview command. It supports `--output table`, `--output json`, and `--output yaml`.

### Creating a config

```bash
llamasitter config init
llamasitter config init --config /tmp/llamasitter.yaml --dry-run
```

Use `--force` if you intentionally want to overwrite an existing file.

### Managing listeners

Listeners are the most important part of the config. They define where LlamaSitter listens locally and where it forwards traffic upstream.

A listener has:

- a unique `name`
- a `listen_addr` such as `127.0.0.1:11435`
- an `upstream_url` such as `http://127.0.0.1:11434`
- optional `default_tags` that fill in attribution fields when the caller does not send explicit headers

Common listener operations:

```bash
llamasitter config listener list
llamasitter config listener show default
llamasitter config listener add \
  --name openwebui \
  --listen-addr 127.0.0.1:11436 \
  --upstream-url http://127.0.0.1:11434 \
  --tag client_type=openwebui \
  --tag client_instance=docker
llamasitter config listener update default --listen-addr 127.0.0.1:11439
llamasitter config listener set-tag default workspace=/Users/me/project
llamasitter config listener unset-tag default workspace
llamasitter config listener remove openwebui --yes
```

Use listener tags when a whole caller class should inherit the same metadata by default. Use request headers when the metadata changes per request or per run.

### Identity headers and default tags

LlamaSitter supports these attribution fields:

- `client_type`
- `client_instance`
- `agent_name`
- `session_id`
- `run_id`
- `workspace`

The matching request headers are:

- `X-LlamaSitter-Client-Type`
- `X-LlamaSitter-Client-Instance`
- `X-LlamaSitter-Agent-Name`
- `X-LlamaSitter-Session-Id`
- `X-LlamaSitter-Run-Id`
- `X-LlamaSitter-Workspace`

Precedence is:

1. explicit request headers
2. listener `default_tags`
3. empty values

That means listener tags are good defaults, but headers always win.

### Managing the dashboard UI

Use these commands to inspect or change the embedded dashboard listener:

```bash
llamasitter config ui show
llamasitter config ui set-listen-addr 127.0.0.1:11439
llamasitter config ui disable
llamasitter config ui enable
```

Disable the UI if you only want the proxy and local storage without the dashboard server.

### Managing storage

Use these commands to inspect or move the SQLite database:

```bash
llamasitter config storage show
llamasitter config storage set-sqlite-path ~/.llamasitter/custom.db
```

If you move storage after data already exists, the CLI updates the path in config. It does not migrate the old database for you.

### Managing privacy settings

By default, LlamaSitter favors metadata-first storage and redacts common sensitive fields.

Inspect the current settings:

```bash
llamasitter config privacy show
```

Common changes:

```bash
llamasitter config privacy set-persist-bodies false
llamasitter config privacy add-redact-header authorization
llamasitter config privacy remove-redact-header authorization
llamasitter config privacy add-redact-json-field messages
llamasitter config privacy remove-redact-json-field messages
```

Use `persist_bodies=false` if you want request accounting without storing prompt and response content.

## macOS Desktop Helpers

If you use the macOS desktop app, the app manages its own config, database, and logs under `~/Library`.

Use these commands to print those paths:

```bash
llamasitter desktop config path
llamasitter desktop db path
llamasitter desktop logs path
```

That lets you target the app's config explicitly from the CLI:

```bash
llamasitter config listener list --config "$(llamasitter desktop config path)"
llamasitter doctor --config "$(llamasitter desktop config path)"
```

These helpers are only available on macOS.

## Shell Completion

Generate completion scripts for the shell you use:

```bash
llamasitter completion zsh > ~/.zsh/completions/_llamasitter
llamasitter completion bash > /usr/local/etc/bash_completion.d/llamasitter
llamasitter completion fish > ~/.config/fish/completions/llamasitter.fish
```

The command also supports PowerShell:

```bash
llamasitter completion powershell
```

## Common Workflows

### Add a second listener for another tool

If one tool should connect through `127.0.0.1:11435` and another should have different default tags on `127.0.0.1:11436`:

```bash
llamasitter config listener add \
  --config llamasitter.yaml \
  --name openwebui \
  --listen-addr 127.0.0.1:11436 \
  --upstream-url http://127.0.0.1:11434 \
  --tag client_type=openwebui \
  --tag client_instance=docker
```

Restart `llamasitter serve`, then point that tool at `127.0.0.1:11436`.

### Move the dashboard to another port

```bash
llamasitter config ui set-listen-addr 127.0.0.1:11439
llamasitter doctor --config llamasitter.yaml
```

Restart the service after the config change.

### Tighten privacy before using real prompts

```bash
llamasitter config privacy set-persist-bodies false
llamasitter config privacy add-redact-header authorization
llamasitter config privacy add-redact-json-field messages
```

### Export data for analysis

```bash
llamasitter export --config llamasitter.yaml --format json --output requests.json
llamasitter export --config llamasitter.yaml --format csv --output requests.csv
```

Use JSON for full-fidelity analysis. Use CSV for quick spreadsheet inspection.

## Troubleshooting

### `doctor` says an upstream is unreachable

Check that Ollama is running and that the listener's `upstream_url` is correct.

The default expected upstream is:

```text
http://127.0.0.1:11434
```

### `stats` or `tail` show no data

Usually this means your client is still talking directly to Ollama instead of the LlamaSitter listener.

Verify that:

- LlamaSitter is running with `llamasitter serve`
- your client is pointed at the listener port, not Ollama's port
- the listener you expect is actually configured

### Attribution fields are empty

Set listener default tags or send `X-LlamaSitter-*` headers with your requests.

### You changed config but behavior did not change

Config edits update the file on disk, but the running service must be restarted to pick up the new settings.

### The desktop app and CLI seem out of sync

You may be editing a local `llamasitter.yaml` while the macOS app is using its own app-managed file. Compare paths with:

```bash
llamasitter config path
llamasitter desktop config path
```

## Getting Help

For exact flags on any command:

```bash
llamasitter --help
llamasitter config --help
llamasitter config listener add --help
llamasitter stats --help
```

Use this guide for workflows and mental models. Use the generated reference under [`reference/cli`](reference/cli/llamasitter.md) when you want the exact syntax for a specific command.
