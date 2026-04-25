# llamasitter config listener set-tag

Set or replace one default tag on a listener

Good listener defaults are usually stable fields like `client_type`, `client_instance`,
and `workspace`. Per-run fields like `session_id` and `run_id` are usually better sent
as request headers.

## Examples

```text
llamasitter config listener set-tag openwebui client_type=openwebui
  llamasitter config listener set-tag openwebui client_instance=docker
  llamasitter config listener set-tag openwebui workspace=/Users/me/project
```

## Usage

```text
Usage:
  llamasitter config listener set-tag NAME KEY=VALUE [flags]

Examples:
  llamasitter config listener set-tag openwebui client_type=openwebui
  llamasitter config listener set-tag openwebui client_instance=docker
  llamasitter config listener set-tag openwebui workspace=/Users/me/project

Flags:
      --dry-run   print the updated config without writing it

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
