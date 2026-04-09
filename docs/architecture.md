# LlamaSitter Architecture

## Purpose

LlamaSitter is a lightweight observability proxy for Ollama. Its job is to capture trustworthy usage and performance data without requiring invasive changes to Ollama itself or to the clients that call it.

The system should remain local-first, low-overhead, and transparent to upstream callers.

## System Overview

```text
Client -> LlamaSitter -> Ollama
```

A client such as OpenClaw, OpenCode, or a custom app sends requests to LlamaSitter instead of directly to Ollama. LlamaSitter forwards those requests upstream, observes the responses, extracts usage metrics, stores normalized usage events, and exposes them via API, UI, and CLI.

## Architectural Goals

- Minimal added latency
- Transparent request and response passthrough
- Accurate token and timing capture
- Reliable streaming support
- Clean multi-agent attribution
- Local persistence with low operational burden
- Enough modularity for later compatibility surfaces

## Major Components

### Proxy Layer

Responsible for:

- Binding local listener ports
- Receiving incoming requests
- Forwarding requests upstream
- Preserving headers, status codes, and body shapes
- Handling both streaming and non-streaming responses

### Identity Resolver

Responsible for:

- Detecting `client_type`
- Resolving `client_instance`
- Attaching `agent_name`
- Grouping `session_id`
- Assigning or preserving `run_id`
- Recording optional tags such as `workspace`

Resolution priority:

1. Explicit request headers
2. Listener defaults
3. Empty values

### Usage Extractor

Responsible for:

- Parsing Ollama-native response payloads
- Extracting prompt and output token counts
- Extracting durations
- Handling final stream payloads
- Normalizing values into a shared usage event model

### Storage Layer

Responsible for:

- Persisting normalized request events
- Persisting flexible tags
- Supporting efficient recent-request and summary queries
- Providing session-oriented aggregation

SQLite is the default storage backend for alpha.

### Analytics Layer

Responsible for:

- Grouping by model, time, client, instance, or session
- Computing totals and averages
- Driving read API responses and dashboard views

### Local API

Responsible for exposing:

- `GET /healthz`
- `GET /readyz`
- `GET /api/requests`
- `GET /api/requests/:id`
- `GET /api/usage/summary`
- `GET /api/usage/timeseries`
- `GET /api/sessions`
- `GET /api/sessions/:id`
- `GET /api/export/requests.csv`
- `GET /api/export/requests.json`

### UI Layer

Responsible for presenting:

- Overview metrics
- Recent requests
- Session summaries
- Model summaries
- Client and instance comparisons

For alpha, the UI is embedded and served by the Go binary.

## Request Lifecycle

### Standard Request Flow

1. Client sends request to LlamaSitter
2. LlamaSitter generates an internal request ID
3. LlamaSitter resolves identity metadata
4. LlamaSitter records request metadata and timing start
5. LlamaSitter forwards the request to Ollama
6. Ollama returns a response
7. LlamaSitter extracts usage metrics
8. LlamaSitter stores a normalized event
9. The response is returned to the client unchanged

### Streaming Request Flow

1. Client opens a streaming request
2. LlamaSitter forwards the request upstream
3. Response chunks are relayed immediately
4. LlamaSitter retains minimal state needed for stream finalization
5. The final chunk is parsed for usage metrics
6. The request record is persisted
7. Stream aborts are marked if completion data never arrives

## Data Model

Each request becomes a normalized event with fields such as:

- `request_id`
- `started_at`
- `finished_at`
- `endpoint`
- `model`
- `http_status`
- `success`
- `aborted`
- `prompt_tokens`
- `output_tokens`
- `total_tokens`
- `request_duration_ms`
- `prompt_eval_duration_ms`
- `eval_duration_ms`
- `request_size_bytes`
- `response_size_bytes`
- `client_type`
- `client_instance`
- `agent_name`
- `session_id`
- `run_id`
- `workspace`

Flexible metadata should be stored as tags to avoid constant schema churn.

## Config Surface

The config supports:

- Multiple listeners
- Upstream URLs
- Default listener tags
- Storage path
- Privacy and redaction settings
- UI enablement and listen address

YAML is preferred for readability.

## Privacy Model

Privacy defaults are essential for local adoption.

Default behavior avoids persisting prompt and response bodies. Users may opt into richer debug logging later, but the default mode stores only metadata, counts, timings, and operational context.

## Failure Modes

Important failure cases include:

- Upstream Ollama unavailable
- Malformed or incomplete responses
- Stream interruption before final usage metrics arrive
- Database write failures
- Config or migration failures

Failures should be recorded locally without breaking request transparency more than necessary.

## Recommended Implementation Order

1. Config loader and validation
2. Single-listener reverse proxy
3. Request IDs and metadata capture
4. Non-streaming usage extraction for `/api/chat`
5. SQLite persistence
6. `doctor` and `tail` CLI tools
7. Streaming support
8. Local read API
9. Embedded overview UI
10. Multi-listener instance attribution
11. Additional route support
