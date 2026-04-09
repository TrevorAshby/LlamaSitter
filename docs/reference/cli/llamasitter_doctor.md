# llamasitter doctor

Validate config, storage, listener upstreams, and UI settings

## Examples

```text
llamasitter doctor --config llamasitter.yaml
  llamasitter doctor --config /Users/me/Library/Application Support/LlamaSitter/llamasitter.yaml --output json
```

## Usage

```text
Usage:
  llamasitter doctor [flags]

Examples:
  llamasitter doctor --config llamasitter.yaml
  llamasitter doctor --config /Users/me/Library/Application Support/LlamaSitter/llamasitter.yaml --output json

Flags:
      --output string   output format: table, json, or yaml (default "table")

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
