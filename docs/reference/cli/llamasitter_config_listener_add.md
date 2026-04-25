# llamasitter config listener add

Add a new listener to the selected config file

Stable fields like `client_type`, `client_instance`, and `workspace` are often good
listener defaults.

## Examples

```text
llamasitter config listener add --name openwebui --listen-addr 127.0.0.1:11436 --upstream-url http://127.0.0.1:11434
  llamasitter config listener add --name openwebui --listen-addr 0.0.0.0:11436 --upstream-url http://127.0.0.1:11434 --tag client_type=openwebui --tag client_instance=docker
  llamasitter config listener add --name agentbox --listen-addr 127.0.0.1:11437 --upstream-url http://127.0.0.1:11434 --tag workspace=/srv/agentbox
```

## Usage

```text
Usage:
  llamasitter config listener add [flags]

Examples:
  llamasitter config listener add --name openwebui --listen-addr 127.0.0.1:11436 --upstream-url http://127.0.0.1:11434
  llamasitter config listener add --name openwebui --listen-addr 0.0.0.0:11436 --upstream-url http://127.0.0.1:11434 --tag client_type=openwebui --tag client_instance=docker
  llamasitter config listener add --name agentbox --listen-addr 127.0.0.1:11437 --upstream-url http://127.0.0.1:11434 --tag workspace=/srv/agentbox

Flags:
      --dry-run               print the updated config without writing it
      --listen-addr string    listener bind address, for example 127.0.0.1:11435
      --name string           listener name
      --tag strings           default tag assignment in KEY=VALUE form (repeatable); common keys: client_type, client_instance, agent_name, session_id, run_id, workspace
      --upstream-url string   upstream Ollama base URL

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
