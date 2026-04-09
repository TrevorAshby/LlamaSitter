# Development Notes

## Local Setup

LlamaSitter is implemented as a single Go binary with embedded static assets. The service loads YAML configuration, opens a SQLite database, runs one or more proxy listeners, and optionally runs a separate read API and UI listener.

## Expected Layout

- `cmd/llamasitter`: program entrypoint
- `desktop/macos`: native macOS Dock app wrapper
- `internal/config`: config loading and validation
- `internal/identity`: request attribution
- `internal/usage`: Ollama response extraction
- `internal/storage`: SQLite persistence and query layer
- `internal/proxy`: transparent proxying and request capture
- `internal/api`: read API and UI handler
- `web/static`: embedded dashboard assets

## Verification

The primary verification loop is:

1. Run unit tests
2. Run integration tests with mocked upstream responses
3. Run end-to-end tests against a real Ollama instance

## Alpha Constraints

- Preserve request and response transparency for `/api/chat`
- Do not persist prompts or responses by default
- Keep attribution explicit rather than heuristic
- Keep the binary self-contained

## macOS Dock App

The macOS desktop shell wraps the existing Go service instead of reimplementing metrics collection. It is split into two app roles:

1. `LlamaSitter.app`: the dashboard window and Dock-facing app
2. `LlamaSitterMenu.app`: an embedded helper bundle that owns the backend process and menu bar icon

The combined desktop flow is:

1. The dashboard app creates or reuses the app-managed config in `~/Library/Application Support/LlamaSitter`
2. The dashboard app launches the embedded menu agent if it is not already running
3. The menu agent launches the bundled backend with `serve --config ...`
4. The dashboard app polls `GET /readyz`
5. The dashboard app loads the embedded dashboard into a native AppKit `WKWebView`
6. The menu agent mirrors a compact live overview into a native menu bar popover

When verifying launch behavior, open the `.app` bundle itself rather than executing `Contents/MacOS/LlamaSitter` directly.

The app bundle is produced by [build-macos-app.sh](/Users/trevorashby/Desktop/LlamaSitter/scripts/build-macos-app.sh).
