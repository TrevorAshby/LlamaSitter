# llamasitter

Observe and manage local Ollama traffic through LlamaSitter

LlamaSitter is a local-first observability proxy for Ollama. It captures usage, timing, model activity, and attribution while remaining transparent to callers.

## Usage

```text
Usage:
  llamasitter [flags]
  llamasitter [command]

Available Commands:
  completion  Generate shell completion scripts
  config      Inspect, validate, and safely mutate LlamaSitter config files
  desktop     Inspect macOS desktop app-managed paths
  doctor      Validate config, storage, listener upstreams, and UI settings
  export      Export captured requests as json or csv
  serve       Start the proxy, storage, API, and dashboard services
  stats       Print aggregate usage metrics from local storage
  tail        Show recent captured requests from local storage
  version     Print LlamaSitter build version metadata

Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")

Use "llamasitter [command] --help" for more information about a command.
```
