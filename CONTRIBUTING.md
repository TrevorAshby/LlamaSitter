# Contributing

Thanks for taking an interest in LlamaSitter.

## Principles

- Keep the proxy path transparent and low-overhead.
- Prefer explicit attribution over heuristics.
- Preserve privacy defaults unless the user opts in.
- Favor small, testable changes over large rewrites.

## Development Workflow

1. Update docs when behavior or interfaces change.
2. Add or update tests with code changes.
3. Keep the binary self-contained.
4. Verify proxy behavior against mocked upstream responses before relying on real Ollama.

## Areas of Focus

- Request transparency and streaming correctness
- Usage extraction accuracy
- SQLite persistence and aggregation queries
- CLI and dashboard usability for local workflows
