# LlamaSitter Development Plan

This repository uses three documents as the source of truth:

- [README](/Users/trevorashby/Desktop/LlamaSitter/README.md) for product overview and scope
- [Architecture](/Users/trevorashby/Desktop/LlamaSitter/docs/architecture.md) for design constraints and interfaces
- [Implementation Checklist](/Users/trevorashby/Desktop/LlamaSitter/ImplementationChecklist.md) for delivery tracking

## Alpha Goal

Ship a single-binary Go service that:

- Proxies native Ollama `/api/chat` traffic
- Preserves upstream request and response behavior
- Captures non-streaming and streaming usage metrics
- Persists normalized request records to SQLite
- Supports explicit attribution via `X-LlamaSitter-*` headers and listener defaults
- Exposes CLI, read API, and embedded local UI surfaces for inspection

## Sequencing

1. Repository baseline and config shape
2. Core proxy path
3. Usage extraction and SQLite persistence
4. Streaming finalization and identity handling
5. CLI and read API
6. Embedded UI
7. Hardening, tests, and release polish

## Deferred Work

The following stay out of alpha unless core proxy behavior is already stable:

- `/api/generate`
- Embeddings routes
- OpenAI-compatible routes
- Heuristic identity inference
- Desktop tray shell
- Advanced analytics beyond the current summary and session views
