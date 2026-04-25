# Identity And Tags

LlamaSitter tracks two related kinds of metadata:

- listener metadata, which identifies the proxy entrypoint that accepted the request
- request identity metadata, which describes the caller, agent, session, run, and workspace

This page is the user-facing reference for how those fields work, where they come from, and when to set them on listeners versus per request.

## Quick Mental Model

- `listener.name` answers: which configured proxy entrypoint accepted this request?
- `client_type` answers: what kind of caller is this?
- `client_instance` answers: which specific install, machine, container, or deployment is this?
- `agent_name` answers: which logical agent is acting?
- `session_id` answers: which conversation or session should related requests be grouped into?
- `run_id` answers: which single execution or job does this request belong to?
- `workspace` answers: which project or working directory is this associated with?

## Listener Name Versus Tags

`listener.name` is part of the config itself. It is not a tag.

Example:

```yaml
listeners:
  - name: openwebui
    listen_addr: "127.0.0.1:11436"
    upstream_url: "http://127.0.0.1:11434"
```

That `name` is stored on every captured request as `listener_name`. It is useful for:

- separating traffic by port or ingress path
- exports and raw analytics
- filtering requests by the entrypoint they came through

Listener `default_tags` are different. They are defaults that fill in request identity fields only when the request does not provide explicit values.

## Supported Identity Fields

LlamaSitter currently recognizes these built-in identity fields:

- `client_type`
- `client_instance`
- `agent_name`
- `session_id`
- `run_id`
- `workspace`

These can be provided either as listener defaults or as per-request headers.

## Exact Request Headers

The matching headers are:

- `X-LlamaSitter-Client-Type`
- `X-LlamaSitter-Client-Instance`
- `X-LlamaSitter-Agent-Name`
- `X-LlamaSitter-Session-Id`
- `X-LlamaSitter-Run-Id`
- `X-LlamaSitter-Workspace`

These names are implemented in [internal/identity/identity.go](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/internal/identity/identity.go).

## Precedence Rules

Resolution order is:

1. explicit request headers
2. listener `default_tags`
3. empty values

That means listener defaults are useful for stable metadata, but headers always win.

## What Belongs On A Listener

Good listener defaults are values that stay stable for most or all traffic on that listener.

Common examples:

- `client_type=openwebui`
- `client_instance=docker`
- `workspace=/srv/agents/project-a`

Example:

```yaml
listeners:
  - name: openwebui
    listen_addr: "127.0.0.1:11436"
    upstream_url: "http://127.0.0.1:11434"
    default_tags:
      client_type: "openwebui"
      client_instance: "docker"
```

This is a good fit when one port or listener consistently represents one caller class.

## What Should Usually Be Set Per Request

These are usually better as headers than listener defaults:

- `agent_name`
- `session_id`
- `run_id`

That is because they often vary from one request or execution to the next.

If you hardcode `session_id` on a listener, every request through that listener will be grouped into the same session. If you hardcode `run_id`, every request through that listener will look like part of one long run. That is usually not what you want.

## Recommended Usage By Field

### `client_type`

Use this for the caller class or framework.

Examples:

- `openwebui`
- `python`
- `desktop-app`
- `codex`

Usually a good listener default.

### `client_instance`

Use this for the specific instance of that caller.

Examples:

- `docker`
- `macos`
- `linux`
- `prod-1`

Usually a good listener default when it maps to one machine, container, or deployment.

### `agent_name`

Use this for the logical agent.

Examples:

- `planner`
- `research-agent`
- `reviewer`

Best as a request header if multiple agents share one listener.

### `session_id`

Use this to group related requests into a conversation or session.

Examples:

- `chat-123`
- `ticket-481-session`

Best as a request header.

### `run_id`

Use this to group requests from a single execution, job, or run.

Examples:

- `run-2026-04-24-001`
- `batch-import-17`

Best as a request header.

### `workspace`

Use this for the current project or working directory.

Examples:

- `/Users/me/project-a`
- `/srv/repos/internal-tool`

This can work either as a listener default or a request header depending on how stable the workspace is.

## Custom Tags

If a listener default contains keys outside the built-in identity set, LlamaSitter stores them as generic request tags instead of mapping them into first-class identity fields.

That lets you attach metadata without changing the schema every time.

Example:

```yaml
default_tags:
  environment: "staging"
  team: "platform"
```

## Examples

### Listener Defaults

```bash
llamasitter config listener set-tag openwebui client_type=openwebui
llamasitter config listener set-tag openwebui client_instance=docker
llamasitter config listener set-tag openwebui workspace=/srv/openwebui
```

### Per-Request Headers

```bash
curl http://127.0.0.1:11435/api/chat \
  -H 'Content-Type: application/json' \
  -H 'X-LlamaSitter-Agent-Name: planner' \
  -H 'X-LlamaSitter-Session-Id: chat-42' \
  -H 'X-LlamaSitter-Run-Id: run-42-a' \
  -d '{
    "model": "llama3",
    "stream": false,
    "messages": [{"role":"user","content":"hello"}]
  }'
```

## Where These Fields Show Up

### `listener.name`

Most visible in:

- config and CLI listener management
- request exports
- analytics filters
- stats listener breakdowns

### `client_type` And `client_instance`

Most visible in:

- dashboard request rows
- dashboard session rows
- stats breakdowns
- request exports

### `agent_name`

Most visible in:

- dashboard request rows
- session summaries
- stats agent breakdowns

### `session_id`

Most visible in:

- dashboard session list
- request rows
- session API endpoints

### `run_id`

Currently most useful in exported or queried request data.

### `workspace`

Useful in exports and future grouping/filtering, especially for multi-project local usage.

## Related References

- [README.md](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/README.md)
- [docs/cli.md](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/docs/cli.md)
- [docs/architecture.md](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/docs/architecture.md)
- [internal/identity/identity.go](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/internal/identity/identity.go)
- [internal/model/types.go](/Users/trevorashby/Desktop/RandomProjects/LlamaSitter.nosync/internal/model/types.go)
