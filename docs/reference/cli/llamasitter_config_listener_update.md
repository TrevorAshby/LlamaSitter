# llamasitter config listener update

Update listener name, bind address, or upstream URL

## Examples

```text
llamasitter config listener update default --listen-addr 127.0.0.1:11439
  llamasitter config listener update openwebui --rename ui-docker --upstream-url http://127.0.0.1:11434
```

## Usage

```text
Usage:
  llamasitter config listener update NAME [flags]

Examples:
  llamasitter config listener update default --listen-addr 127.0.0.1:11439
  llamasitter config listener update openwebui --rename ui-docker --upstream-url http://127.0.0.1:11434

Flags:
      --dry-run               print the updated config without writing it
      --listen-addr string    set a new bind address
      --rename string         rename the listener
      --upstream-url string   set a new upstream URL

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
