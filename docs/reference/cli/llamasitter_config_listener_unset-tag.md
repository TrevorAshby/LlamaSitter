# llamasitter config listener unset-tag

Remove one default tag from a listener

This only removes the listener default. It does not affect explicit
`X-LlamaSitter-*` headers sent by callers.

## Examples

```text
llamasitter config listener unset-tag openwebui client_type
```

## Usage

```text
Usage:
  llamasitter config listener unset-tag NAME KEY [flags]

Examples:
  llamasitter config listener unset-tag openwebui client_type

Flags:
      --dry-run   print the updated config without writing it

Global Flags:
  -c, --config string   path to config file (default "llamasitter.yaml")
```
