# llamasitter tail

Show recent captured requests from local storage

## Examples

```text
llamasitter tail --config llamasitter.yaml -n 20
  llamasitter tail --config llamasitter.yaml --output json
```

## Usage

```text
Usage:
  llamasitter tail [flags]

Examples:
  llamasitter tail --config llamasitter.yaml -n 20
  llamasitter tail --config llamasitter.yaml --output json

Flags:
  -n, --n int           number of requests to show (default 20)
      --output string   output format: table, json, or yaml (default "table")

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
