# llamasitter export

Export captured requests as json or csv

## Examples

```text
llamasitter export --config llamasitter.yaml --format json
  llamasitter export --config llamasitter.yaml --format csv --output requests.csv
```

## Usage

```text
Usage:
  llamasitter export [flags]

Examples:
  llamasitter export --config llamasitter.yaml --format json
  llamasitter export --config llamasitter.yaml --format csv --output requests.csv

Flags:
      --format string   export format: json or csv (default "json")
      --output string   output path or - for stdout (default "-")

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
