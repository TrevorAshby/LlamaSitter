package model

import "time"

type Identity struct {
	ClientType     string `json:"client_type"`
	ClientInstance string `json:"client_instance"`
	AgentName      string `json:"agent_name"`
	SessionID      string `json:"session_id"`
	RunID          string `json:"run_id"`
	Workspace      string `json:"workspace"`
}

type RequestEvent struct {
	RequestID               string    `json:"request_id"`
	ListenerName            string    `json:"listener_name"`
	StartedAt               time.Time `json:"started_at"`
	FinishedAt              time.Time `json:"finished_at"`
	Method                  string    `json:"method"`
	Endpoint                string    `json:"endpoint"`
	Model                   string    `json:"model"`
	HTTPStatus              int       `json:"http_status"`
	Success                 bool      `json:"success"`
	Aborted                 bool      `json:"aborted"`
	Stream                  bool      `json:"stream"`
	ErrorMessage            string    `json:"error_message,omitempty"`
	PromptTokens            int64     `json:"prompt_tokens"`
	OutputTokens            int64     `json:"output_tokens"`
	TotalTokens             int64     `json:"total_tokens"`
	RequestDurationMs       int64     `json:"request_duration_ms"`
	PromptEvalDurationMs    int64     `json:"prompt_eval_duration_ms"`
	EvalDurationMs          int64     `json:"eval_duration_ms"`
	RequestSizeBytes        int64     `json:"request_size_bytes"`
	ResponseSizeBytes       int64     `json:"response_size_bytes"`
	UpstreamTotalDurationMs int64     `json:"upstream_total_duration_ms"`
	Identity
	Tags map[string]string `json:"tags,omitempty"`
}

type RequestFilter struct {
	Limit          int
	Offset         int
	Endpoint       string
	Model          string
	ClientType     string
	ClientInstance string
	SessionID      string
	ListenerName   string
	StartedAfter   time.Time
	StartedBefore  time.Time
}

type BreakdownRow struct {
	Key                  string  `json:"key"`
	RequestCount         int64   `json:"request_count"`
	PromptTokens         int64   `json:"prompt_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	TotalTokens          int64   `json:"total_tokens"`
	AvgRequestDurationMs float64 `json:"avg_request_duration_ms"`
}

type BucketBreakdownEntry struct {
	Name         string `json:"name"`
	RequestCount int64  `json:"request_count"`
	PromptTokens int64  `json:"prompt_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

type UsageSummary struct {
	RequestCount         int64          `json:"request_count"`
	SuccessCount         int64          `json:"success_count"`
	AbortedCount         int64          `json:"aborted_count"`
	ActiveSessionCount   int64          `json:"active_session_count"`
	PromptTokens         int64          `json:"prompt_tokens"`
	OutputTokens         int64          `json:"output_tokens"`
	TotalTokens          int64          `json:"total_tokens"`
	AvgRequestDurationMs float64        `json:"avg_request_duration_ms"`
	ByModel              []BreakdownRow `json:"by_model"`
	ByClientType         []BreakdownRow `json:"by_client_type"`
	ByClientInstance     []BreakdownRow `json:"by_client_instance"`
	ByAgentName          []BreakdownRow `json:"by_agent_name"`
}

type TimeBucket struct {
	BucketStart             time.Time              `json:"bucket_start"`
	BucketEnd               time.Time              `json:"bucket_end"`
	Label                   string                 `json:"label"`
	RequestCount            int64                  `json:"request_count"`
	SuccessCount            int64                  `json:"success_count"`
	AbortedCount            int64                  `json:"aborted_count"`
	PromptTokens            int64                  `json:"prompt_tokens"`
	OutputTokens            int64                  `json:"output_tokens"`
	TotalTokens             int64                  `json:"total_tokens"`
	AvgRequestDurationMs    float64                `json:"avg_request_duration_ms"`
	ModelBreakdown          []BucketBreakdownEntry `json:"model_breakdown,omitempty"`
	ClientTypeBreakdown     []BucketBreakdownEntry `json:"client_type_breakdown,omitempty"`
	ClientInstanceBreakdown []BucketBreakdownEntry `json:"client_instance_breakdown,omitempty"`
	AgentNameBreakdown      []BucketBreakdownEntry `json:"agent_name_breakdown,omitempty"`
}

type HeatmapCell struct {
	Weekday                 int                    `json:"weekday"`
	Hour                    int                    `json:"hour"`
	RequestCount            int64                  `json:"request_count"`
	TotalTokens             int64                  `json:"total_tokens"`
	ModelBreakdown          []BucketBreakdownEntry `json:"model_breakdown,omitempty"`
	ClientTypeBreakdown     []BucketBreakdownEntry `json:"client_type_breakdown,omitempty"`
	ClientInstanceBreakdown []BucketBreakdownEntry `json:"client_instance_breakdown,omitempty"`
	AgentNameBreakdown      []BucketBreakdownEntry `json:"agent_name_breakdown,omitempty"`
}

type SessionSummary struct {
	SessionID       string    `json:"session_id"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	RequestCount    int64     `json:"request_count"`
	PromptTokens    int64     `json:"prompt_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	TotalTokens     int64     `json:"total_tokens"`
	ClientType      string    `json:"client_type"`
	ClientInstance  string    `json:"client_instance"`
	AgentName       string    `json:"agent_name"`
	Workspace       string    `json:"workspace"`
	ClientTypes     []string  `json:"client_types,omitempty"`
	ClientInstances []string  `json:"client_instances,omitempty"`
	AgentNames      []string  `json:"agent_names,omitempty"`
	Workspaces      []string  `json:"workspaces,omitempty"`
}

type SessionDetail struct {
	Session  SessionSummary `json:"session"`
	Requests []RequestEvent `json:"requests"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}
