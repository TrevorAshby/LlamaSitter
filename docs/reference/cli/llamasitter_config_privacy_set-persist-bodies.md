# llamasitter config privacy set-persist-bodies

Enable or disable prompt/response body persistence

## Examples

```text
llamasitter config privacy set-persist-bodies false
  llamasitter config privacy set-persist-bodies true --dry-run
```

## Usage

```text
Usage:
  llamasitter config privacy set-persist-bodies true|false [flags]

Examples:
  llamasitter config privacy set-persist-bodies false
  llamasitter config privacy set-persist-bodies true --dry-run

Flags:
      --dry-run   print the updated config without writing it

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
