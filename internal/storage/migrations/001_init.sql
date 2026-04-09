CREATE TABLE IF NOT EXISTS requests (
    request_id TEXT PRIMARY KEY,
    listener_name TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL,
    method TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL,
    success INTEGER NOT NULL,
    aborted INTEGER NOT NULL,
    stream INTEGER NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    request_duration_ms INTEGER NOT NULL DEFAULT 0,
    prompt_eval_duration_ms INTEGER NOT NULL DEFAULT 0,
    eval_duration_ms INTEGER NOT NULL DEFAULT 0,
    upstream_total_duration_ms INTEGER NOT NULL DEFAULT 0,
    request_size_bytes INTEGER NOT NULL DEFAULT 0,
    response_size_bytes INTEGER NOT NULL DEFAULT 0,
    client_type TEXT NOT NULL DEFAULT '',
    client_instance TEXT NOT NULL DEFAULT '',
    agent_name TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    run_id TEXT NOT NULL DEFAULT '',
    workspace TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS request_tags (
    request_id TEXT NOT NULL,
    tag_key TEXT NOT NULL,
    tag_value TEXT NOT NULL,
    PRIMARY KEY (request_id, tag_key),
    FOREIGN KEY (request_id) REFERENCES requests(request_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    client_type TEXT NOT NULL DEFAULT '',
    client_instance TEXT NOT NULL DEFAULT '',
    agent_name TEXT NOT NULL DEFAULT '',
    workspace TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_requests_started_at ON requests(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_requests_model ON requests(model);
CREATE INDEX IF NOT EXISTS idx_requests_client_type ON requests(client_type);
CREATE INDEX IF NOT EXISTS idx_requests_client_instance ON requests(client_instance);
CREATE INDEX IF NOT EXISTS idx_requests_session_id ON requests(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen_at ON sessions(last_seen_at DESC);
