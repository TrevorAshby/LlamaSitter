# llamasitter config privacy

Inspect and update privacy and redaction settings

## Usage

```text
Usage:
  llamasitter config privacy [flags]
  llamasitter config privacy [command]

Available Commands:
  add-redact-header        Add one header name to redact_headers
  add-redact-json-field    Add one field name to redact_json_fields
  remove-redact-header     Remove one header name from redact_headers
  remove-redact-json-field Remove one field name from redact_json_fields
  set-persist-bodies       Enable or disable prompt/response body persistence
  show                     Show privacy and redaction settings

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")

Use "llamasitter config privacy [command] --help" for more information about a command.
```
