# llamasitter config

Inspect, validate, and safely mutate LlamaSitter config files

## Usage

```text
Usage:
  llamasitter config [flags]
  llamasitter config [command]

Available Commands:
  init        Create a default config file at the selected path
  listener    List, inspect, add, update, tag, and remove listeners
  path        Print the resolved path of the target config file
  privacy     Inspect and update privacy and redaction settings
  storage     Inspect and update storage settings
  ui          Inspect and update dashboard UI settings
  validate    Validate the selected config file without contacting upstreams
  view        Render the current config as yaml, json, or a summary table

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")

Use "llamasitter config [command] --help" for more information about a command.
```
