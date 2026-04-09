package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevorashby/llamasitter/internal/model"
)

func TestSQLiteStoreInsertAndQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "llamasitter.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	started := time.Now().UTC().Add(-2 * time.Minute)
	event := &model.RequestEvent{
		RequestID:               "req-test-1",
		ListenerName:            "alpha",
		StartedAt:               started,
		FinishedAt:              started.Add(2 * time.Second),
		Method:                  "POST",
		Endpoint:                "/api/chat",
		Model:                   "llama3",
		HTTPStatus:              200,
		Success:                 true,
		Stream:                  false,
		PromptTokens:            12,
		OutputTokens:            9,
		TotalTokens:             21,
		RequestDurationMs:       2000,
		UpstreamTotalDurationMs: 1800,
		Identity: model.Identity{
			ClientType:     "opencode",
			ClientInstance: "local-1",
			SessionID:      "session-1",
		},
		Tags: map[string]string{
			"environment": "test",
		},
	}

	if err := store.InsertRequest(ctx, event); err != nil {
		t.Fatalf("insert request: %v", err)
	}

	items, err := store.ListRequests(ctx, model.RequestFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 request, got %d", len(items))
	}
	if items[0].Tags["environment"] != "test" {
		t.Fatalf("expected persisted tag")
	}

	summary, err := store.UsageSummary(ctx, model.RequestFilter{})
	if err != nil {
		t.Fatalf("usage summary: %v", err)
	}
	if summary.RequestCount != 1 || summary.TotalTokens != 21 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	session, err := store.GetSession(ctx, "session-1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.RequestCount != 1 {
		t.Fatalf("expected session request count 1, got %d", session.RequestCount)
	}
}

func TestGetSessionClearsSingularAgentWhenSessionHasMultipleAgents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "llamasitter.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	started := time.Now().UTC().Add(-1 * time.Minute)
	events := []*model.RequestEvent{
		{
			RequestID:         "req-agent-1",
			ListenerName:      "alpha",
			StartedAt:         started,
			FinishedAt:        started.Add(1500 * time.Millisecond),
			Method:            "POST",
			Endpoint:          "/api/chat",
			Model:             "llama3",
			HTTPStatus:        200,
			Success:           true,
			PromptTokens:      10,
			OutputTokens:      5,
			TotalTokens:       15,
			RequestDurationMs: 1500,
			Identity: model.Identity{
				ClientType:     "opencode",
				ClientInstance: "local-1",
				AgentName:      "agent-a",
				SessionID:      "session-mixed",
			},
		},
		{
			RequestID:         "req-agent-2",
			ListenerName:      "alpha",
			StartedAt:         started.Add(2 * time.Second),
			FinishedAt:        started.Add(3500 * time.Millisecond),
			Method:            "POST",
			Endpoint:          "/api/chat",
			Model:             "llama3",
			HTTPStatus:        200,
			Success:           true,
			PromptTokens:      8,
			OutputTokens:      7,
			TotalTokens:       15,
			RequestDurationMs: 1500,
			Identity: model.Identity{
				ClientType:     "opencode",
				ClientInstance: "local-1",
				AgentName:      "agent-b",
				SessionID:      "session-mixed",
			},
		},
	}

	for _, event := range events {
		if err := store.InsertRequest(ctx, event); err != nil {
			t.Fatalf("insert request %s: %v", event.RequestID, err)
		}
	}

	session, err := store.GetSession(ctx, "session-mixed")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if session.AgentName != "" {
		t.Fatalf("expected ambiguous singular agent name to be empty, got %q", session.AgentName)
	}
	if len(session.AgentNames) != 2 || session.AgentNames[0] != "agent-a" || session.AgentNames[1] != "agent-b" {
		t.Fatalf("expected both agent names, got %#v", session.AgentNames)
	}
	if session.ClientType != "opencode" {
		t.Fatalf("expected stable singular client type, got %q", session.ClientType)
	}
	if len(session.ClientTypes) != 1 || session.ClientTypes[0] != "opencode" {
		t.Fatalf("expected single client type slice, got %#v", session.ClientTypes)
	}
}
