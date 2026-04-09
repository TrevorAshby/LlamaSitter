package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevorashby/llamasitter/internal/model"
	"github.com/trevorashby/llamasitter/internal/storage"
)

func newAPITestHandler(t *testing.T) (*storage.SQLiteStore, http.Handler) {
	t.Helper()

	ctx := context.Background()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "llamasitter.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	server := &Server{
		store:  store,
		logger: nil,
		assets: http.NotFoundHandler(),
	}

	t.Cleanup(func() {
		_ = store.Close()
	})

	return store, server.routes()
}

func insertAPIEvent(t *testing.T, store *storage.SQLiteStore, event *model.RequestEvent) {
	t.Helper()
	if err := store.InsertRequest(context.Background(), event); err != nil {
		t.Fatalf("insert request %s: %v", event.RequestID, err)
	}
}

func apiEvent(id string, startedAt time.Time, totalTokens int64, sessionID string) *model.RequestEvent {
	return &model.RequestEvent{
		RequestID:               id,
		ListenerName:            "default",
		StartedAt:               startedAt,
		FinishedAt:              startedAt.Add(2 * time.Second),
		Method:                  "POST",
		Endpoint:                "/api/chat",
		Model:                   "llama3",
		HTTPStatus:              200,
		Success:                 true,
		PromptTokens:            totalTokens / 2,
		OutputTokens:            totalTokens - (totalTokens / 2),
		TotalTokens:             totalTokens,
		RequestDurationMs:       2000,
		UpstreamTotalDurationMs: 1800,
		Identity: model.Identity{
			ClientType:     "opencode",
			ClientInstance: "inst-1",
			AgentName:      "agent-1",
			SessionID:      sessionID,
		},
	}
}

func TestUsageEndpointsRespectWindowBounds(t *testing.T) {
	t.Parallel()

	store, handler := newAPITestHandler(t)

	now := time.Date(2026, time.April, 8, 18, 0, 0, 0, time.UTC)
	insertAPIEvent(t, store, apiEvent("req-1", now.Add(-4*time.Hour), 20, "session-a"))
	insertAPIEvent(t, store, apiEvent("req-2", now.Add(-2*time.Hour), 10, "session-a"))
	insertAPIEvent(t, store, apiEvent("req-old", now.Add(-3*24*time.Hour), 99, "session-old"))

	query := "?started_after=2026-04-07T18:00:00Z&started_before=2026-04-08T18:00:00Z"

	var summary model.UsageSummary
	performJSONRequest(t, handler, "/api/usage/summary"+query, &summary, http.StatusOK)
	if summary.RequestCount != 2 || summary.TotalTokens != 30 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	var requests model.ListResponse[model.RequestEvent]
	performJSONRequest(t, handler, "/api/requests"+query+"&limit=10", &requests, http.StatusOK)
	if requests.Count != 2 {
		t.Fatalf("expected 2 requests, got %d", requests.Count)
	}

	var sessions model.ListResponse[model.SessionSummary]
	performJSONRequest(t, handler, "/api/sessions"+query+"&limit=10", &sessions, http.StatusOK)
	if sessions.Count != 1 || sessions.Items[0].SessionID != "session-a" {
		t.Fatalf("unexpected sessions payload: %+v", sessions)
	}

	var timeseries model.ListResponse[model.TimeBucket]
	performJSONRequest(t, handler, "/api/usage/timeseries"+query+"&range=day", &timeseries, http.StatusOK)
	if timeseries.Count != 24 {
		t.Fatalf("expected 24 buckets, got %d", timeseries.Count)
	}
	var bucketRequests int64
	for _, bucket := range timeseries.Items {
		bucketRequests += bucket.RequestCount
		if len(bucket.ModelBreakdown) != 0 {
			t.Fatalf("expected aggregate-only timeseries response by default, got %+v", bucket)
		}
	}
	if bucketRequests != summary.RequestCount {
		t.Fatalf("bucket request total = %d, want %d", bucketRequests, summary.RequestCount)
	}

	var heatmap model.ListResponse[model.HeatmapCell]
	performJSONRequest(t, handler, "/api/usage/heatmap"+query+"&range=day&tz_offset_minutes=0", &heatmap, http.StatusOK)
	if heatmap.Count != 7*24 {
		t.Fatalf("expected 168 heatmap cells, got %d", heatmap.Count)
	}
	for _, cell := range heatmap.Items {
		if len(cell.ModelBreakdown) != 0 {
			t.Fatalf("expected aggregate-only heatmap response by default, got %+v", cell)
		}
	}
}

func TestUsageEndpointsCanIncludeBreakdowns(t *testing.T) {
	t.Parallel()

	store, handler := newAPITestHandler(t)

	now := time.Date(2026, time.April, 8, 18, 0, 0, 0, time.UTC)
	eventA := apiEvent("req-a", now.Add(-4*time.Hour), 20, "session-a")
	eventA.Model = "llama3"
	eventA.ClientType = "codex"
	eventA.ClientInstance = "inst-a"
	eventA.AgentName = "agent-a"
	insertAPIEvent(t, store, eventA)

	eventB := apiEvent("req-b", now.Add(-2*time.Hour), 10, "session-b")
	eventB.Model = "mistral"
	eventB.ClientType = "openwebui"
	eventB.ClientInstance = "inst-b"
	eventB.AgentName = "agent-b"
	insertAPIEvent(t, store, eventB)

	query := "?started_after=2026-04-07T18:00:00Z&started_before=2026-04-08T18:00:00Z"

	var timeseries model.ListResponse[model.TimeBucket]
	performJSONRequest(t, handler, "/api/usage/timeseries"+query+"&range=day&include_breakdowns=true", &timeseries, http.StatusOK)
	var populatedBuckets int
	for _, bucket := range timeseries.Items {
		if bucket.RequestCount == 0 {
			continue
		}
		populatedBuckets++
		if len(bucket.ModelBreakdown) == 0 || len(bucket.ClientTypeBreakdown) == 0 || len(bucket.ClientInstanceBreakdown) == 0 || len(bucket.AgentNameBreakdown) == 0 {
			t.Fatalf("expected full timeseries breakdowns, got %+v", bucket)
		}
	}
	if populatedBuckets != 2 {
		t.Fatalf("expected 2 populated buckets, got %d", populatedBuckets)
	}

	var heatmap model.ListResponse[model.HeatmapCell]
	performJSONRequest(t, handler, "/api/usage/heatmap"+query+"&range=day&tz_offset_minutes=0&include_breakdowns=true", &heatmap, http.StatusOK)
	var populatedCells int
	for _, cell := range heatmap.Items {
		if cell.RequestCount == 0 {
			continue
		}
		populatedCells++
		if len(cell.ModelBreakdown) == 0 || len(cell.ClientTypeBreakdown) == 0 || len(cell.ClientInstanceBreakdown) == 0 || len(cell.AgentNameBreakdown) == 0 {
			t.Fatalf("expected full heatmap breakdowns, got %+v", cell)
		}
	}
	if populatedCells != 2 {
		t.Fatalf("expected 2 populated cells, got %d", populatedCells)
	}
}

func TestRequestsRejectInvalidTimeBounds(t *testing.T) {
	t.Parallel()

	_, handler := newAPITestHandler(t)
	performJSONRequest(t, handler, "/api/requests?started_after=not-a-time", nil, http.StatusBadRequest)
}

func performJSONRequest(t *testing.T, handler http.Handler, path string, target any, statusCode int) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != statusCode {
		t.Fatalf("expected %d for %s, got %d with body %s", statusCode, path, recorder.Code, recorder.Body.String())
	}

	if target == nil {
		return
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
