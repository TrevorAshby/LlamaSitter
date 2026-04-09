# LlamaSitter Implementation Checklist

## Phase 0: Repository Baseline

- [x] Repository exists
- [x] MIT license is chosen
- [x] Add a product README
- [x] Add architecture documentation
- [x] Add an implementation checklist
- [ ] Add `CONTRIBUTING.md`
- [ ] Add `CODE_OF_CONDUCT.md`
- [ ] Add issue templates
- [ ] Add pull request template

## Phase 1: Project Scaffold

- [ ] Create Go module
- [ ] Add `cmd/llamasitter/main.go`
- [ ] Add internal package layout
- [ ] Add config package
- [ ] Add logging and application wiring
- [ ] Add CLI command structure
- [ ] Add build scripts
- [ ] Add sample config

## Phase 2: Configuration and Validation

- [ ] Define YAML config schema
- [ ] Support multiple listeners
- [ ] Support default listener tags
- [ ] Add upstream URL validation
- [ ] Add storage path validation
- [ ] Add startup config error reporting

## Phase 3: Core Reverse Proxy

- [ ] Implement listener-backed proxy server
- [ ] Add request ID generation
- [ ] Forward `/api/chat` transparently
- [ ] Preserve status codes and headers
- [ ] Capture request timing start and end
- [ ] Record request metadata

## Phase 4: Usage Extraction and Persistence

- [ ] Parse non-streaming Ollama chat responses
- [ ] Extract prompt token count
- [ ] Extract output token count
- [ ] Extract durations
- [ ] Normalize into usage event model
- [ ] Add SQLite connection layer
- [ ] Add migrations system
- [ ] Create `requests` and `request_tags` tables
- [ ] Add indexes and insert methods

## Phase 5: Streaming and Identity

- [ ] Support streaming `/api/chat`
- [ ] Relay chunks immediately
- [ ] Parse final stream object
- [ ] Record final usage metrics
- [ ] Detect aborted streams
- [ ] Add support for identity headers
- [ ] Add listener-level default tags
- [ ] Test identity precedence rules

## Phase 6: CLI and Read API

- [ ] Implement `llamasitter serve`
- [ ] Implement `llamasitter doctor`
- [ ] Implement `llamasitter stats`
- [ ] Implement `llamasitter tail`
- [ ] Implement `llamasitter export`
- [ ] Add health endpoints
- [ ] Add requests, usage summary, sessions, and export endpoints

## Phase 7: Embedded UI

- [ ] Create embedded frontend assets
- [ ] Build overview page
- [ ] Build recent requests table
- [ ] Build sessions view
- [ ] Build per-model summary view
- [ ] Build per-instance comparison view

## Phase 8: Hardening and Release Prep

- [ ] Add unit tests for config parsing
- [ ] Add unit tests for usage extraction
- [ ] Add integration tests for proxy behavior
- [ ] Add migration tests
- [ ] Add end-to-end tests against real Ollama
- [ ] Add local development docs
- [ ] Add build and release workflow

## Deferred Until Post-Alpha

- [ ] Add `/api/generate`
- [ ] Add embeddings routes
- [ ] Add configurable redaction
- [ ] Add OpenAI-compatible routes
- [ ] Evaluate tray UI and advanced analytics
