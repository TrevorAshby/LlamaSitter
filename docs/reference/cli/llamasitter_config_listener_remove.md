# llamasitter config listener remove

Remove one listener from the selected config file

## Examples

```text
llamasitter config listener remove openwebui --yes
  llamasitter config listener remove openwebui --yes --dry-run
```

## Usage

```text
Usage:
  llamasitter config listener remove NAME [flags]

Examples:
  llamasitter config listener remove openwebui --yes
  llamasitter config listener remove openwebui --yes --dry-run

Flags:
      --dry-run   print the updated config without writing it
      --yes       confirm destructive removal

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
