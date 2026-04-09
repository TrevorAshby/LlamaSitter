# llamasitter config listener

List, inspect, add, update, tag, and remove listeners

## Usage

```text
Usage:
  llamasitter config listener [flags]
  llamasitter config listener [command]

Available Commands:
  add         Add a new listener to the selected config file
  list        List configured listeners
  remove      Remove one listener from the selected config file
  set-tag     Set or replace one default tag on a listener
  show        Show one configured listener
  unset-tag   Remove one default tag from a listener
  update      Update listener name, bind address, or upstream URL

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")

Use "llamasitter config listener [command] --help" for more information about a command.
```
