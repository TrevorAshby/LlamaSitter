# LlamaSitter

LlamaSitter is a lightweight, local-first observability layer for Ollama. It sits between Ollama-powered clients and the Ollama server, proxies requests transparently, and records token usage, timing metrics, request patterns, model activity, and per-agent attribution.

It is designed for developers who use Ollama through tools like OpenClaw, OpenCode, custom local agents, and direct app integrations, and want trustworthy visibility into what Ollama is actually processing.

## Why LlamaSitter?

When Ollama is used behind agent frameworks or wrappers, it becomes harder to answer practical questions like:

- How many input tokens did this request actually use?
- How many output tokens were generated?
- Which agent, instance, or run consumed those tokens?
- Does framework-side reporting match Ollama's real counters?
- Which model or workflow is causing latency or token spikes?

LlamaSitter answers those questions without requiring invasive changes to Ollama or to callers.

## Core Features

- Transparent local reverse proxy for Ollama
- Tracks prompt tokens, output tokens, total tokens, durations, and sizes
- Supports per-client, per-instance, per-agent, per-session, and per-run attribution
- Works with multiple concurrent local agent frameworks
- Uses SQLite for lightweight local persistence
- Exposes a local API, embedded dashboard, and CLI
- Defaults to metadata-only persistence for privacy
- Leaves room for later compatibility routes and richer analytics

## Alpha Scope

The current implementation targets a usable alpha:

- Native Ollama `/api/chat` support
- Streaming and non-streaming request capture
- SQLite persistence with embedded migrations
- Explicit identity headers plus listener-level default tags
- `serve`, `doctor`, `stats`, `tail`, and `export` CLI commands
- Read-only local API for requests, summaries, sessions, and exports
- Embedded local web UI for overview metrics and recent activity

The following are intentionally deferred until after the core proxy path is stable:

- `/api/generate`
- Embeddings routes
- OpenAI-compatible routes
- Desktop tray shells
- Heuristic auto-attribution

## Identity Model

LlamaSitter separates identity into distinct layers:

- `client_type`: framework or caller type such as `openclaw`, `opencode`, or `python`
- `client_instance`: a specific instance such as `openclaw1` or `openclaw2`
- `agent_name`: the logical agent such as `research-agent`
- `session_id`: a logical conversation or session grouping
- `run_id`: a single job or execution grouping
- `workspace`: an optional working directory or project tag

Headers override defaults. The current precedence is:

1. Explicit `X-LlamaSitter-*` headers
2. Listener default tags from config
3. Empty values

## Example Configuration

See [config.example.yaml](/Users/trevorashby/Desktop/LlamaSitter/config.example.yaml) for a full sample. A minimal setup looks like this:

```yaml
listeners:
  - name: default
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"
    default_tags:
      client_type: "unknown"
      client_instance: "default"

storage:
  sqlite_path: "~/.llamasitter/llamasitter.db"

ui:
  enabled: true
  listen_addr: "127.0.0.1:11438"
```

## CLI

Planned command surface:

- `llamasitter serve`
- `llamasitter doctor`
- `llamasitter stats`
- `llamasitter tail`
- `llamasitter export --format csv`

## Performance Impact

LlamaSitter is designed to be transparent enough that developers can leave it in the request path without meaningfully slowing local Ollama workflows. To validate that claim, this repo now includes repeatable benchmark harnesses under `benchmarks/` plus raw CSV outputs and documentation-ready figures under `benchmarks/results/` and `benchmarks/figures/`.

### Study Design

Two crossover benchmarks were run locally against the same machine, using the same model and the same request payloads:

- Model: `qwen3-vl:8b`
- Direct endpoint: `http://127.0.0.1:11434/api/chat`
- Proxy endpoint: `http://127.0.0.1:11435/api/chat`
- Prompt sizes: 50 lengths from 10 to 500 filler words
- Design: crossover, where each prompt length is tested once as `direct -> proxy` and once as `proxy -> direct`
- Sample size:
  - Non-streaming: 100 successful paired measurements
  - Streaming: 100 successful paired measurements

This crossover design matters because it removes most of the “second request is faster” warm-cache bias that showed up in the earlier sequential benchmark. The resulting numbers are a much better estimate of actual proxy overhead.

The scripts and raw outputs used for this section are:

- [Non-streaming crossover CSV](benchmarks/results/ollama_vs_llamasitter_crossover_latency_20260409T004023Z.csv)
- [Streaming crossover CSV](benchmarks/results/ollama_vs_llamasitter_streaming_crossover_latency_20260409T005129Z.csv)
- [Non-streaming benchmark harness](benchmarks/run_proxy_latency_benchmark.py)
- [Streaming benchmark harness](benchmarks/run_streaming_proxy_latency_benchmark.py)
- [Plotting script](benchmarks/plot_latency_benchmarks.py)

### Benchmark Figures

![Prompt-size latency curves for direct Ollama versus LlamaSitter](benchmarks/figures/llamasitter_latency_prompt_curves.png)

![Crossover benchmark overhead summary for direct Ollama versus LlamaSitter](benchmarks/figures/llamasitter_latency_overhead_summary.png)

### Summary Values

| Scenario | Metric | Direct Ollama | Via LlamaSitter | Paired proxy delta |
| --- | --- | ---: | ---: | ---: |
| Non-streaming crossover | Mean completion time | 246.535 ms | 249.423 ms | +2.888 ms |
| Non-streaming crossover | Median completion time | 225.970 ms | 228.982 ms | +2.596 ms |
| Streaming crossover | Mean time to first chunk | 122.363 ms | 123.082 ms | +0.719 ms |
| Streaming crossover | Median time to first chunk | 99.862 ms | 101.121 ms | +0.460 ms |
| Streaming crossover | Mean total completion time | 2408.633 ms | 2400.247 ms | -8.386 ms |
| Streaming crossover | Median total completion time | 2389.098 ms | 2394.878 ms | -13.126 ms |

The paired proxy delta column comes directly from the crossover CSV summaries. Negative values do not mean the proxy is inherently faster; they mean the measured difference is small enough that normal run-to-run variance and residual cache effects can outweigh it. The practical takeaway from both studies is that LlamaSitter adds little to no meaningful latency in this local setup, with measured overhead landing around a few milliseconds.

If you want to reproduce the same figures, run the benchmark scripts again and then rerun the plotting script:

```bash
python3 benchmarks/run_proxy_latency_benchmark.py --design crossover --count 50
python3 benchmarks/run_streaming_proxy_latency_benchmark.py --design crossover --count 50
python3 benchmarks/plot_latency_benchmarks.py
```

## macOS App

A native macOS desktop wrapper now lives under [desktop/macos](/Users/trevorashby/Desktop/LlamaSitter/desktop/macos). The visible [LlamaSitter.app](/Users/trevorashby/Desktop/LlamaSitter/build/macos/LlamaSitter.app) acts as the dashboard window, while an embedded background menu bar agent owns the bundled `llamasitter serve` process and keeps the status icon alive even after the dashboard window is closed.

To build a local `.app` bundle:

```bash
bash ./scripts/build-macos-app.sh
```

That produces:

```text
build/macos/LlamaSitter.app
```

Launch the bundle itself, not the inner Mach-O:

```bash
open /Users/trevorashby/Desktop/LlamaSitter/build/macos/LlamaSitter.app
```

Do not launch `Contents/MacOS/LlamaSitter` directly unless you are intentionally debugging the bundle internals. The embedded background agent is launched automatically from the app bundle when needed.

## Project Docs

- [Architecture](/Users/trevorashby/Desktop/LlamaSitter/docs/architecture.md)
- [Development Notes](/Users/trevorashby/Desktop/LlamaSitter/docs/development.md)
- [Implementation Checklist](/Users/trevorashby/Desktop/LlamaSitter/ImplementationChecklist.md)
- [Development Plan](/Users/trevorashby/Desktop/LlamaSitter/DevelopmentPlan.md)

## Status

LlamaSitter is in early implementation.
