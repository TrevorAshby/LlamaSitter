package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevorashby/llamasitter/internal/model"
)

func newAnalyticsTestStore(t *testing.T) (*SQLiteStore, context.Context) {
	t.Helper()

	ctx := context.Background()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "llamasitter.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return store, ctx
}

func insertEvent(t *testing.T, ctx context.Context, store *SQLiteStore, event *model.RequestEvent) {
	t.Helper()
	if err := store.InsertRequest(ctx, event); err != nil {
		t.Fatalf("insert request %s: %v", event.RequestID, err)
	}
}

func makeEvent(id string, startedAt time.Time, sessionID string, totalTokens int64, modelName string, identity model.Identity) *model.RequestEvent {
	if modelName == "" {
		modelName = "llama3"
	}
	return &model.RequestEvent{
		RequestID:               id,
		ListenerName:            "default",
		StartedAt:               startedAt,
		FinishedAt:              startedAt.Add(1500 * time.Millisecond),
		Method:                  "POST",
		Endpoint:                "/api/chat",
		Model:                   modelName,
		HTTPStatus:              200,
		Success:                 true,
		PromptTokens:            totalTokens / 2,
		OutputTokens:            totalTokens - (totalTokens / 2),
		TotalTokens:             totalTokens,
		RequestDurationMs:       1500,
		UpstreamTotalDurationMs: 1300,
		Identity: model.Identity{
			ClientType:     identity.ClientType,
			ClientInstance: identity.ClientInstance,
			AgentName:      identity.AgentName,
			SessionID:      sessionID,
			RunID:          identity.RunID,
			Workspace:      identity.Workspace,
		},
	}
}

func TestAnalyticsQueriesRespectTimeWindow(t *testing.T) {
	t.Parallel()

	store, ctx := newAnalyticsTestStore(t)
	defer store.Close()

	now := time.Date(2026, time.April, 8, 18, 0, 0, 0, time.UTC)
	insertEvent(t, ctx, store, makeEvent("req-old", now.Add(-8*24*time.Hour), "session-old", 9, "llama3", model.Identity{
		ClientType:     "opencode",
		ClientInstance: "inst-old",
		AgentName:      "agent-old",
	}))
	insertEvent(t, ctx, store, makeEvent("req-day-1", now.Add(-6*time.Hour), "session-a", 20, "llama3", model.Identity{
		ClientType:     "opencode",
		ClientInstance: "inst-a",
		AgentName:      "agent-a",
	}))
	insertEvent(t, ctx, store, makeEvent("req-day-2", now.Add(-2*time.Hour), "session-a", 10, "mistral", model.Identity{
		ClientType:     "opencode",
		ClientInstance: "inst-a",
		AgentName:      "agent-b",
	}))
	insertEvent(t, ctx, store, makeEvent("req-week", now.Add(-26*time.Hour), "session-b", 30, "llama3", model.Identity{
		ClientType:     "codex",
		ClientInstance: "inst-b",
		AgentName:      "agent-c",
	}))

	filter := model.RequestFilter{
		StartedAfter:  now.Add(-24 * time.Hour),
		StartedBefore: now,
		Limit:         10,
	}

	summary, err := store.UsageSummary(ctx, filter)
	if err != nil {
		t.Fatalf("usage summary: %v", err)
	}
	if summary.RequestCount != 2 {
		t.Fatalf("expected 2 requests in window, got %d", summary.RequestCount)
	}
	if summary.TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens, got %d", summary.TotalTokens)
	}
	if summary.ActiveSessionCount != 1 {
		t.Fatalf("expected 1 active session, got %d", summary.ActiveSessionCount)
	}
	if len(summary.ByAgentName) != 2 {
		t.Fatalf("expected 2 agent breakdown rows, got %d", len(summary.ByAgentName))
	}
	if len(summary.ByListenerName) != 1 || summary.ByListenerName[0].Key != "default" {
		t.Fatalf("expected listener breakdown for default, got %#v", summary.ByListenerName)
	}

	requests, err := store.ListRequests(ctx, model.RequestFilter{
		StartedAfter:  filter.StartedAfter,
		StartedBefore: filter.StartedBefore,
		Limit:         0,
	})
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	sessions, err := store.ListSessions(ctx, filter)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session in window, got %d", len(sessions))
	}
	if sessions[0].SessionID != "session-a" {
		t.Fatalf("expected session-a, got %q", sessions[0].SessionID)
	}
	if sessions[0].AgentName != "" {
		t.Fatalf("expected singular agent name to clear for mixed session, got %q", sessions[0].AgentName)
	}
	if len(sessions[0].AgentNames) != 2 {
		t.Fatalf("expected 2 agent names, got %#v", sessions[0].AgentNames)
	}

	timeseries, err := store.UsageTimeseries(ctx, filter, "day", true)
	if err != nil {
		t.Fatalf("usage timeseries: %v", err)
	}
	if len(timeseries) != 24 {
		t.Fatalf("expected 24 hourly buckets, got %d", len(timeseries))
	}

	var bucketRequests int64
	var bucketTokens int64
	for _, bucket := range timeseries {
		bucketRequests += bucket.RequestCount
		bucketTokens += bucket.TotalTokens
	}
	if bucketRequests != summary.RequestCount {
		t.Fatalf("bucket request total = %d, want %d", bucketRequests, summary.RequestCount)
	}
	if bucketTokens != summary.TotalTokens {
		t.Fatalf("bucket token total = %d, want %d", bucketTokens, summary.TotalTokens)
	}
	var bucketsWithBreakdowns int
	for _, bucket := range timeseries {
		if len(bucket.ModelBreakdown) == 0 && bucket.RequestCount > 0 {
			t.Fatalf("expected model breakdowns for populated bucket %+v", bucket)
		}
		if len(bucket.ModelBreakdown) > 0 {
			bucketsWithBreakdowns++
		}
	}
	if bucketsWithBreakdowns != 2 {
		t.Fatalf("expected 2 populated buckets with breakdowns, got %d", bucketsWithBreakdowns)
	}

	heatmap, err := store.UsageHeatmap(ctx, filter, 0, true)
	if err != nil {
		t.Fatalf("usage heatmap: %v", err)
	}
	if len(heatmap) != 7*24 {
		t.Fatalf("expected 168 heatmap cells, got %d", len(heatmap))
	}
	var heatmapRequests int64
	for _, cell := range heatmap {
		heatmapRequests += cell.RequestCount
	}
	if heatmapRequests != summary.RequestCount {
		t.Fatalf("heatmap request total = %d, want %d", heatmapRequests, summary.RequestCount)
	}
	var nonEmptyCells int
	for _, cell := range heatmap {
		if cell.RequestCount == 0 {
			if len(cell.ModelBreakdown) != 0 {
				t.Fatalf("expected empty heatmap cell to omit breakdowns, got %+v", cell)
			}
			continue
		}
		nonEmptyCells++
		if len(cell.ModelBreakdown) == 0 || len(cell.ClientInstanceBreakdown) == 0 || len(cell.AgentNameBreakdown) == 0 {
			t.Fatalf("expected populated heatmap cell to include breakdowns, got %+v", cell)
		}
	}
	if nonEmptyCells != 2 {
		t.Fatalf("expected 2 populated heatmap cells, got %d", nonEmptyCells)
	}
}

func TestUsageTimeseriesMonthBuildsStableWeeklyBuckets(t *testing.T) {
	t.Parallel()

	store, ctx := newAnalyticsTestStore(t)
	defer store.Close()

	end := time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC)
	start := end.Add(-35 * 24 * time.Hour)

	for i := 0; i < 5; i++ {
		startedAt := start.Add(time.Duration(i*7+1) * 24 * time.Hour)
		insertEvent(t, ctx, store, makeEvent(
			"req-month-"+string(rune('a'+i)),
			startedAt,
			"session-month",
			int64(10+i),
			"llama3",
			model.Identity{ClientType: "opencode", ClientInstance: "inst-month", AgentName: "agent-month"},
		))
	}

	items, err := store.UsageTimeseries(ctx, model.RequestFilter{
		StartedAfter:  start,
		StartedBefore: end,
	}, "month", false)
	if err != nil {
		t.Fatalf("usage timeseries month: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("expected 5 weekly buckets, got %d", len(items))
	}

	for i, item := range items {
		if item.BucketEnd.Sub(item.BucketStart) != 7*24*time.Hour {
			t.Fatalf("bucket %d duration = %s, want 7d", i, item.BucketEnd.Sub(item.BucketStart))
		}
		if item.RequestCount != 1 {
			t.Fatalf("bucket %d request count = %d, want 1", i, item.RequestCount)
		}
	}
}

func TestAnalyticsBreakdownsCanBeSkipped(t *testing.T) {
	t.Parallel()

	store, ctx := newAnalyticsTestStore(t)
	defer store.Close()

	now := time.Date(2026, time.April, 8, 18, 0, 0, 0, time.UTC)
	insertEvent(t, ctx, store, makeEvent("req-1", now.Add(-2*time.Hour), "session-a", 25, "llama3", model.Identity{
		ClientType:     "codex",
		ClientInstance: "inst-a",
		AgentName:      "agent-a",
	}))

	filter := model.RequestFilter{
		StartedAfter:  now.Add(-24 * time.Hour),
		StartedBefore: now,
	}

	timeseries, err := store.UsageTimeseries(ctx, filter, "day", false)
	if err != nil {
		t.Fatalf("usage timeseries without breakdowns: %v", err)
	}
	for _, bucket := range timeseries {
		if len(bucket.ModelBreakdown) != 0 || len(bucket.ClientTypeBreakdown) != 0 || len(bucket.ClientInstanceBreakdown) != 0 || len(bucket.AgentNameBreakdown) != 0 {
			t.Fatalf("expected no bucket breakdowns when disabled, got %+v", bucket)
		}
	}

	heatmap, err := store.UsageHeatmap(ctx, filter, 0, false)
	if err != nil {
		t.Fatalf("usage heatmap without breakdowns: %v", err)
	}
	for _, cell := range heatmap {
		if len(cell.ModelBreakdown) != 0 || len(cell.ClientTypeBreakdown) != 0 || len(cell.ClientInstanceBreakdown) != 0 || len(cell.AgentNameBreakdown) != 0 {
			t.Fatalf("expected no heatmap breakdowns when disabled, got %+v", cell)
		}
	}
}
