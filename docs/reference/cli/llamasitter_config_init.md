# llamasitter config init

Create a default config file at the selected path

## Examples

```text
llamasitter config init
  llamasitter config init --config /tmp/llamasitter.yaml --dry-run
```

## Usage

```text
Usage:
  llamasitter config init [flags]

Examples:
  llamasitter config init
  llamasitter config init --config /tmp/llamasitter.yaml --dry-run

Flags:
      --dry-run   print the generated config without writing it
      --force     overwrite an existing config file

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
