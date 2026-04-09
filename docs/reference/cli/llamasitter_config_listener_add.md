# llamasitter config listener add

Add a new listener to the selected config file

## Examples

```text
llamasitter config listener add --name openwebui --listen-addr 127.0.0.1:11436 --upstream-url http://127.0.0.1:11434
  llamasitter config listener add --name openwebui --listen-addr 0.0.0.0:11436 --upstream-url http://127.0.0.1:11434 --tag client_type=openwebui --tag client_instance=docker
```

## Usage

```text
Usage:
  llamasitter config listener add [flags]

Examples:
  llamasitter config listener add --name openwebui --listen-addr 127.0.0.1:11436 --upstream-url http://127.0.0.1:11434
  llamasitter config listener add --name openwebui --listen-addr 0.0.0.0:11436 --upstream-url http://127.0.0.1:11434 --tag client_type=openwebui --tag client_instance=docker

Flags:
      --dry-run               print the updated config without writing it
      --listen-addr string    listener bind address, for example 127.0.0.1:11435
      --name string           listener name
      --tag strings           default tag assignment in KEY=VALUE form (repeatable)
      --upstream-url string   upstream Ollama base URL

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
