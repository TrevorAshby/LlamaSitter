# CLI Guide

LlamaSitter includes a nested CLI for running the service, inspecting local data, and safely editing config files without hand-editing YAML unless you want to.

## Conventions

- `--config PATH` selects the config file to read or mutate. If omitted, commands use local `llamasitter.yaml`.
- Mutating config commands support `--dry-run` so you can preview the exact YAML before writing it.
- Destructive listener removal requires `--yes`.
- Inspect commands generally support `--output table|json|yaml`.

## Initialize A Config

Create a default config in the current directory:

```bash
llamasitter config init
```

Preview the generated config without writing it:

```bash
llamasitter config init --dry-run
```

Validate a config file without starting the service:

```bash
llamasitter config validate --config llamasitter.yaml
```

## Add Or Remove A Listener

List the current listeners:

```bash
llamasitter config listener list --config llamasitter.yaml
```

Add a new listener for another interface:

```bash
llamasitter config listener add --config llamasitter.yaml \
  --name openwebui \
  --listen-addr 127.0.0.1:11436 \
  --upstream-url http://127.0.0.1:11434 \
  --tag client_type=openwebui \
  --tag client_instance=docker
```

Preview the result first:

```bash
llamasitter config listener add --config llamasitter.yaml \
  --name openwebui \
  --listen-addr 127.0.0.1:11436 \
  --upstream-url http://127.0.0.1:11434 \
  --dry-run
```

Update an existing listener:

```bash
llamasitter config listener update default --listen-addr 127.0.0.1:11439
```

Remove a listener:

```bash
llamasitter config listener remove openwebui --yes
```

## Change The UI Port

Inspect the current UI settings:

```bash
llamasitter config ui show
```

Move the dashboard UI listener to a new address:

```bash
llamasitter config ui set-listen-addr 127.0.0.1:11439
```

Disable or re-enable the UI:

```bash
llamasitter config ui disable
llamasitter config ui enable
```

## Inspect Storage And Privacy

Show the SQLite path:

```bash
llamasitter config storage show
```

Move the SQLite database:

```bash
llamasitter config storage set-sqlite-path ~/.llamasitter/custom.db
```

Show privacy settings:

```bash
llamasitter config privacy show
```

Update privacy defaults:

```bash
llamasitter config privacy set-persist-bodies false
llamasitter config privacy add-redact-header authorization
llamasitter config privacy add-redact-json-field messages
```

## Runtime And Data Inspection

Validate config, storage, upstream connectivity, and UI settings:

```bash
llamasitter doctor --config llamasitter.yaml
```

Print aggregate stats:

```bash
llamasitter stats --config llamasitter.yaml
```

Tail recent requests:

```bash
llamasitter tail --config llamasitter.yaml -n 20
```

Export captured requests:

```bash
llamasitter export --config llamasitter.yaml --format csv --output requests.csv
```

## macOS Desktop Paths

The desktop app keeps its config, database, and logs under app-managed macOS paths. The CLI makes those paths explicit:

```bash
llamasitter desktop config path
llamasitter desktop db path
llamasitter desktop logs path
```

That makes it easy to target the desktop app config deliberately:

```bash
llamasitter config listener list --config "$(llamasitter desktop config path)"
```

## Shell Completion

Generate shell completions:

```bash
llamasitter completion zsh > ~/.zsh/completions/_llamasitter
llamasitter completion bash > /usr/local/etc/bash_completion.d/llamasitter
```

## Reference

Generated command reference pages live under [docs/reference/cli](/Users/trevorashby/Desktop/LlamaSitter/docs/reference/cli/llamasitter.md).
