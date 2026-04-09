# llamasitter config listener set-tag

Set or replace one default tag on a listener

## Examples

```text
llamasitter config listener set-tag openwebui client_type=openwebui
  llamasitter config listener set-tag openwebui workspace=/Users/me/project
```

## Usage

```text
Usage:
  llamasitter config listener set-tag NAME KEY=VALUE [flags]

Examples:
  llamasitter config listener set-tag openwebui client_type=openwebui
  llamasitter config listener set-tag openwebui workspace=/Users/me/project

Flags:
      --dry-run   print the updated config without writing it

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
