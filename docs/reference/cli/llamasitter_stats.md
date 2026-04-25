# llamasitter stats

Print aggregate usage metrics from local storage

The default terminal view renders a dense SSH-friendly dashboard with totals, recent windows,
trend lines, breakdowns, top sessions, and recent requests. JSON and YAML remain summary-only.

## Examples

```text
llamasitter stats --config llamasitter.yaml
  llamasitter stats --config llamasitter.yaml --output json
```

## Usage

```text
Usage:
  llamasitter stats [flags]

Examples:
  llamasitter stats --config llamasitter.yaml
  llamasitter stats --config llamasitter.yaml --output json

Flags:
      --output string   output format: table, json, or yaml (default "table")

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
