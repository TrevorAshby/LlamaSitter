package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/trevorashby/llamasitter/internal/analytics"
	"github.com/trevorashby/llamasitter/internal/model"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	store := &SQLiteStore{db: db}
	if err := store.applyPragmas(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		applied, err := s.migrationApplied(ctx, filepath.Base(name))
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", name, err)
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %q: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %q: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)
		`, filepath.Base(name), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %q: %w", name, err)
		}
	}

	return nil
}

func (s *SQLiteStore) InsertRequest(ctx context.Context, event *model.RequestEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert request: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO requests (
			request_id, listener_name, started_at, finished_at, method, endpoint, model, http_status,
			success, aborted, stream, error_message, prompt_tokens, output_tokens, total_tokens,
			request_duration_ms, prompt_eval_duration_ms, eval_duration_ms, upstream_total_duration_ms,
			request_size_bytes, response_size_bytes, client_type, client_instance, agent_name,
			session_id, run_id, workspace
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.RequestID, event.ListenerName, formatTime(event.StartedAt), formatTime(event.FinishedAt),
		event.Method, event.Endpoint, event.Model, event.HTTPStatus, boolToInt(event.Success),
		boolToInt(event.Aborted), boolToInt(event.Stream), event.ErrorMessage, event.PromptTokens,
		event.OutputTokens, event.TotalTokens, event.RequestDurationMs, event.PromptEvalDurationMs,
		event.EvalDurationMs, event.UpstreamTotalDurationMs, event.RequestSizeBytes,
		event.ResponseSizeBytes, event.ClientType, event.ClientInstance, event.AgentName,
		event.SessionID, event.RunID, event.Workspace,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert request: %w", err)
	}

	for key, value := range event.Tags {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO request_tags (request_id, tag_key, tag_value) VALUES (?, ?, ?)
		`, event.RequestID, key, value); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert request tag %q: %w", key, err)
		}
	}

	if strings.TrimSpace(event.SessionID) != "" {
		if err := upsertSession(ctx, tx, event); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit request insert: %w", err)
	}

	return nil
}

func (s *SQLiteStore) ListRequests(ctx context.Context, filter model.RequestFilter) ([]model.RequestEvent, error) {
	where, args := buildRequestWhere(filter)
	query := `
		SELECT
			request_id, listener_name, started_at, finished_at, method, endpoint, model, http_status,
			success, aborted, stream, error_message, prompt_tokens, output_tokens, total_tokens,
			request_duration_ms, prompt_eval_duration_ms, eval_duration_ms, upstream_total_duration_ms,
			request_size_bytes, response_size_bytes, client_type, client_instance, agent_name,
			session_id, run_id, workspace
		FROM requests
	` + where + `
		ORDER BY started_at DESC
	`

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query requests: %w", err)
	}
	defer rows.Close()

	var items []model.RequestEvent
	for rows.Next() {
		item, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}

	for i := range items {
		tags, err := s.loadTags(ctx, items[i].RequestID)
		if err != nil {
			return nil, err
		}
		items[i].Tags = tags
	}

	return items, nil
}

func (s *SQLiteStore) GetRequest(ctx context.Context, id string) (*model.RequestEvent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			request_id, listener_name, started_at, finished_at, method, endpoint, model, http_status,
			success, aborted, stream, error_message, prompt_tokens, output_tokens, total_tokens,
			request_duration_ms, prompt_eval_duration_ms, eval_duration_ms, upstream_total_duration_ms,
			request_size_bytes, response_size_bytes, client_type, client_instance, agent_name,
			session_id, run_id, workspace
		FROM requests
		WHERE request_id = ?
	`, id)

	item, err := scanRequest(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("get request %q: %w", id, err)
	}

	item.Tags, err = s.loadTags(ctx, item.RequestID)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *SQLiteStore) UsageSummary(ctx context.Context, filter model.RequestFilter) (*model.UsageSummary, error) {
	where, args := buildRequestWhere(filter)

	summary := &model.UsageSummary{}
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN aborted = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(COUNT(DISTINCT CASE WHEN session_id <> '' THEN session_id END), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(AVG(request_duration_ms), 0)
		FROM requests
	`+where, args...).Scan(
		&summary.RequestCount,
		&summary.SuccessCount,
		&summary.AbortedCount,
		&summary.ActiveSessionCount,
		&summary.PromptTokens,
		&summary.OutputTokens,
		&summary.TotalTokens,
		&summary.AvgRequestDurationMs,
	); err != nil {
		return nil, fmt.Errorf("query usage summary: %w", err)
	}

	var err error
	if summary.ByModel, err = s.breakdown(ctx, "model", filter); err != nil {
		return nil, err
	}
	if summary.ByClientType, err = s.breakdown(ctx, "client_type", filter); err != nil {
		return nil, err
	}
	if summary.ByClientInstance, err = s.breakdown(ctx, "client_instance", filter); err != nil {
		return nil, err
	}
	if summary.ByAgentName, err = s.breakdown(ctx, "agent_name", filter); err != nil {
		return nil, err
	}

	return summary, nil
}

type bucketBreakdownAccumulator struct {
	RequestCount int64
	PromptTokens int64
	OutputTokens int64
	TotalTokens  int64
}

type bucketBreakdownGroups struct {
	Model          map[string]*bucketBreakdownAccumulator
	ClientType     map[string]*bucketBreakdownAccumulator
	ClientInstance map[string]*bucketBreakdownAccumulator
	AgentName      map[string]*bucketBreakdownAccumulator
}

func (s *SQLiteStore) UsageTimeseries(ctx context.Context, filter model.RequestFilter, rangeName string, includeBreakdowns bool) ([]model.TimeBucket, error) {
	rows, err := s.analyticsRows(ctx, filter)
	if err != nil {
		return nil, err
	}

	bucketWindows := analytics.BucketWindows(rangeName, filter.StartedAfter, filter.StartedBefore)
	items := make([]model.TimeBucket, len(bucketWindows))
	durationTotals := make([]float64, len(bucketWindows))
	breakdownGroups := make([]bucketBreakdownGroups, len(bucketWindows))

	for i, window := range bucketWindows {
		items[i] = model.TimeBucket{
			BucketStart: window.Start,
			BucketEnd:   window.End,
			Label:       analytics.BucketLabel(rangeName, window.Start),
		}
	}

	for _, row := range rows {
		index := bucketIndex(rangeName, filter.StartedAfter, row.StartedAt)
		if index < 0 || index >= len(items) {
			continue
		}

		items[index].RequestCount++
		if row.Success {
			items[index].SuccessCount++
		}
		if row.Aborted {
			items[index].AbortedCount++
		}
		items[index].PromptTokens += row.PromptTokens
		items[index].OutputTokens += row.OutputTokens
		items[index].TotalTokens += row.TotalTokens
		durationTotals[index] += float64(row.RequestDurationMs)

		if includeBreakdowns {
			group := &breakdownGroups[index]
			addBucketBreakdown(group.model(), row.Model, row)
			addBucketBreakdown(group.clientType(), row.ClientType, row)
			addBucketBreakdown(group.clientInstance(), row.ClientInstance, row)
			addBucketBreakdown(group.agentName(), row.AgentName, row)
		}
	}

	for i := range items {
		if items[i].RequestCount > 0 {
			items[i].AvgRequestDurationMs = durationTotals[i] / float64(items[i].RequestCount)
		}
		if includeBreakdowns {
			items[i].ModelBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].Model)
			items[i].ClientTypeBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].ClientType)
			items[i].ClientInstanceBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].ClientInstance)
			items[i].AgentNameBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].AgentName)
		}
	}

	return items, nil
}

func (s *SQLiteStore) UsageHeatmap(ctx context.Context, filter model.RequestFilter, tzOffsetMinutes int, includeBreakdowns bool) ([]model.HeatmapCell, error) {
	rows, err := s.analyticsRows(ctx, filter)
	if err != nil {
		return nil, err
	}

	cells := make([]model.HeatmapCell, 0, 7*24)
	indexBySlot := make(map[int]int, 7*24)
	breakdownGroups := make([]bucketBreakdownGroups, 0, 7*24)
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			slot := weekday*24 + hour
			indexBySlot[slot] = len(cells)
			cells = append(cells, model.HeatmapCell{
				Weekday: weekday,
				Hour:    hour,
			})
			breakdownGroups = append(breakdownGroups, bucketBreakdownGroups{})
		}
	}

	location := time.FixedZone("dashboard", -tzOffsetMinutes*60)
	for _, row := range rows {
		local := row.StartedAt.In(location)
		slot := int(local.Weekday())*24 + local.Hour()
		index := indexBySlot[slot]
		cells[index].RequestCount++
		cells[index].TotalTokens += row.TotalTokens
		if includeBreakdowns {
			group := &breakdownGroups[index]
			addBucketBreakdown(group.model(), row.Model, row)
			addBucketBreakdown(group.clientType(), row.ClientType, row)
			addBucketBreakdown(group.clientInstance(), row.ClientInstance, row)
			addBucketBreakdown(group.agentName(), row.AgentName, row)
		}
	}

	if includeBreakdowns {
		for i := range cells {
			cells[i].ModelBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].Model)
			cells[i].ClientTypeBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].ClientType)
			cells[i].ClientInstanceBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].ClientInstance)
			cells[i].AgentNameBreakdown = finalizeBucketBreakdowns(breakdownGroups[i].AgentName)
		}
	}

	return cells, nil
}

func (s *SQLiteStore) ListSessions(ctx context.Context, filter model.RequestFilter) ([]model.SessionSummary, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	where, args := buildRequestWhere(filter)
	if where == "" {
		where = " WHERE session_id <> ''"
	} else {
		where += " AND session_id <> ''"
	}
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			session_id,
			MIN(started_at) AS first_seen_at,
			MAX(finished_at) AS last_seen_at,
			COUNT(*) AS request_count,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			'' AS client_type,
			'' AS client_instance,
			'' AS agent_name,
			'' AS workspace
		FROM requests
	`+where+`
		GROUP BY session_id
		ORDER BY total_tokens DESC, request_count DESC, last_seen_at DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var items []model.SessionSummary
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	for i := range items {
		if err := s.hydrateSessionSummary(ctx, &items[i], filter); err != nil {
			return nil, err
		}
	}

	return items, nil
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (*model.SessionSummary, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			session_id, first_seen_at, last_seen_at, request_count, prompt_tokens, output_tokens,
			total_tokens, client_type, client_instance, agent_name, workspace
		FROM sessions
		WHERE session_id = ?
	`, id)

	item, err := scanSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("get session %q: %w", id, err)
	}

	if err := s.hydrateSessionSummary(ctx, &item, model.RequestFilter{}); err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *SQLiteStore) breakdown(ctx context.Context, column string, filter model.RequestFilter) ([]model.BreakdownRow, error) {
	where, args := buildRequestWhere(filter)
	if where == "" {
		where = " WHERE " + column + " <> ''"
	} else {
		where += " AND " + column + " <> ''"
	}

	query := fmt.Sprintf(`
		SELECT
			%s,
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(AVG(request_duration_ms), 0)
		FROM requests
		%s
		GROUP BY %s
		ORDER BY COALESCE(SUM(total_tokens), 0) DESC, COUNT(*) DESC, %s ASC
		LIMIT 10
	`, column, where, column, column)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s breakdown: %w", column, err)
	}
	defer rows.Close()

	var items []model.BreakdownRow
	for rows.Next() {
		var item model.BreakdownRow
		if err := rows.Scan(
			&item.Key,
			&item.RequestCount,
			&item.PromptTokens,
			&item.OutputTokens,
			&item.TotalTokens,
			&item.AvgRequestDurationMs,
		); err != nil {
			return nil, fmt.Errorf("scan %s breakdown: %w", column, err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s breakdown: %w", column, err)
	}

	return items, nil
}

func (g *bucketBreakdownGroups) model() map[string]*bucketBreakdownAccumulator {
	if g.Model == nil {
		g.Model = map[string]*bucketBreakdownAccumulator{}
	}
	return g.Model
}

func (g *bucketBreakdownGroups) clientType() map[string]*bucketBreakdownAccumulator {
	if g.ClientType == nil {
		g.ClientType = map[string]*bucketBreakdownAccumulator{}
	}
	return g.ClientType
}

func (g *bucketBreakdownGroups) clientInstance() map[string]*bucketBreakdownAccumulator {
	if g.ClientInstance == nil {
		g.ClientInstance = map[string]*bucketBreakdownAccumulator{}
	}
	return g.ClientInstance
}

func (g *bucketBreakdownGroups) agentName() map[string]*bucketBreakdownAccumulator {
	if g.AgentName == nil {
		g.AgentName = map[string]*bucketBreakdownAccumulator{}
	}
	return g.AgentName
}

func addBucketBreakdown(target map[string]*bucketBreakdownAccumulator, name string, row analyticsRow) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	item := target[name]
	if item == nil {
		item = &bucketBreakdownAccumulator{}
		target[name] = item
	}

	item.RequestCount++
	item.PromptTokens += row.PromptTokens
	item.OutputTokens += row.OutputTokens
	item.TotalTokens += row.TotalTokens
}

func finalizeBucketBreakdowns(items map[string]*bucketBreakdownAccumulator) []model.BucketBreakdownEntry {
	if len(items) == 0 {
		return nil
	}

	rows := make([]model.BucketBreakdownEntry, 0, len(items))
	for name, item := range items {
		rows = append(rows, model.BucketBreakdownEntry{
			Name:         name,
			RequestCount: item.RequestCount,
			PromptTokens: item.PromptTokens,
			OutputTokens: item.OutputTokens,
			TotalTokens:  item.TotalTokens,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalTokens != rows[j].TotalTokens {
			return rows[i].TotalTokens > rows[j].TotalTokens
		}
		if rows[i].RequestCount != rows[j].RequestCount {
			return rows[i].RequestCount > rows[j].RequestCount
		}
		return rows[i].Name < rows[j].Name
	})

	return rows
}

func (s *SQLiteStore) loadTags(ctx context.Context, requestID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tag_key, tag_value
		FROM request_tags
		WHERE request_id = ?
		ORDER BY tag_key ASC
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("query request tags: %w", err)
	}
	defer rows.Close()

	tags := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan request tag: %w", err)
		}
		tags[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, nil
	}
	return tags, nil
}

type analyticsRow struct {
	StartedAt         time.Time
	PromptTokens      int64
	OutputTokens      int64
	TotalTokens       int64
	RequestDurationMs int64
	Success           bool
	Aborted           bool
	Model             string
	ClientType        string
	ClientInstance    string
	AgentName         string
}

func (s *SQLiteStore) analyticsRows(ctx context.Context, filter model.RequestFilter) ([]analyticsRow, error) {
	where, args := buildRequestWhere(filter)

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			started_at,
			prompt_tokens,
			output_tokens,
			total_tokens,
			request_duration_ms,
			success,
			aborted,
			model,
			client_type,
			client_instance,
			agent_name
		FROM requests
	`+where+`
		ORDER BY started_at ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query analytics rows: %w", err)
	}
	defer rows.Close()

	var items []analyticsRow
	for rows.Next() {
		var startedAt string
		var success int64
		var aborted int64
		var item analyticsRow
		if err := rows.Scan(
			&startedAt,
			&item.PromptTokens,
			&item.OutputTokens,
			&item.TotalTokens,
			&item.RequestDurationMs,
			&success,
			&aborted,
			&item.Model,
			&item.ClientType,
			&item.ClientInstance,
			&item.AgentName,
		); err != nil {
			return nil, fmt.Errorf("scan analytics row: %w", err)
		}
		item.StartedAt = parseTime(startedAt)
		item.Success = success == 1
		item.Aborted = aborted == 1
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics rows: %w", err)
	}

	return items, nil
}

func (s *SQLiteStore) migrationApplied(ctx context.Context, version string) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM schema_migrations
		WHERE version = ?
	`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %q: %w", version, err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}

	for _, stmt := range pragmas {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", stmt, err)
		}
	}

	return nil
}

func upsertSession(ctx context.Context, tx *sql.Tx, event *model.RequestEvent) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (
			session_id, first_seen_at, last_seen_at, request_count, prompt_tokens,
			output_tokens, total_tokens, client_type, client_instance, agent_name, workspace
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			first_seen_at = CASE
				WHEN sessions.first_seen_at <= excluded.first_seen_at THEN sessions.first_seen_at
				ELSE excluded.first_seen_at
			END,
			last_seen_at = CASE
				WHEN sessions.last_seen_at >= excluded.last_seen_at THEN sessions.last_seen_at
				ELSE excluded.last_seen_at
			END,
			request_count = sessions.request_count + 1,
			prompt_tokens = sessions.prompt_tokens + excluded.prompt_tokens,
			output_tokens = sessions.output_tokens + excluded.output_tokens,
			total_tokens = sessions.total_tokens + excluded.total_tokens,
			client_type = CASE WHEN excluded.client_type <> '' THEN excluded.client_type ELSE sessions.client_type END,
			client_instance = CASE WHEN excluded.client_instance <> '' THEN excluded.client_instance ELSE sessions.client_instance END,
			agent_name = CASE WHEN excluded.agent_name <> '' THEN excluded.agent_name ELSE sessions.agent_name END,
			workspace = CASE WHEN excluded.workspace <> '' THEN excluded.workspace ELSE sessions.workspace END
	`, event.SessionID, formatTime(event.StartedAt), formatTime(event.FinishedAt), 1,
		event.PromptTokens, event.OutputTokens, event.TotalTokens, event.ClientType,
		event.ClientInstance, event.AgentName, event.Workspace); err != nil {
		return fmt.Errorf("upsert session %q: %w", event.SessionID, err)
	}
	return nil
}

func (s *SQLiteStore) hydrateSessionSummary(ctx context.Context, item *model.SessionSummary, filter model.RequestFilter) error {
	if item == nil || strings.TrimSpace(item.SessionID) == "" {
		return nil
	}

	clientTypes, err := s.loadDistinctSessionValues(ctx, item.SessionID, "client_type", filter)
	if err != nil {
		return err
	}
	clientInstances, err := s.loadDistinctSessionValues(ctx, item.SessionID, "client_instance", filter)
	if err != nil {
		return err
	}
	agentNames, err := s.loadDistinctSessionValues(ctx, item.SessionID, "agent_name", filter)
	if err != nil {
		return err
	}
	workspaces, err := s.loadDistinctSessionValues(ctx, item.SessionID, "workspace", filter)
	if err != nil {
		return err
	}

	item.ClientTypes = clientTypes
	item.ClientInstances = clientInstances
	item.AgentNames = agentNames
	item.Workspaces = workspaces

	item.ClientType = singleValueOrEmpty(clientTypes)
	item.ClientInstance = singleValueOrEmpty(clientInstances)
	item.AgentName = singleValueOrEmpty(agentNames)
	item.Workspace = singleValueOrEmpty(workspaces)

	return nil
}

func (s *SQLiteStore) loadDistinctSessionValues(ctx context.Context, sessionID, column string, filter model.RequestFilter) ([]string, error) {
	switch column {
	case "client_type", "client_instance", "agent_name", "workspace":
	default:
		return nil, fmt.Errorf("unsupported session attribution column %q", column)
	}

	filter.SessionID = sessionID
	where, args := buildRequestWhere(filter)
	if where == "" {
		where = " WHERE " + column + " <> ''"
	} else {
		where += " AND " + column + " <> ''"
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT %s
		FROM requests
		%s
		ORDER BY %s ASC
	`, column, where, column)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query distinct %s for session %q: %w", column, sessionID, err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan distinct %s for session %q: %w", column, sessionID, err)
		}
		values = append(values, value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate distinct %s for session %q: %w", column, sessionID, err)
	}

	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}

func bucketIndex(rangeName string, windowStart, value time.Time) int {
	spec := analytics.ResolveRange(rangeName)
	if windowStart.IsZero() {
		return -1
	}

	diff := value.UTC().Sub(windowStart.UTC())
	if diff < 0 {
		return -1
	}

	index := int(math.Floor(diff.Seconds() / spec.BucketDuration.Seconds()))
	if index < 0 || index >= spec.BucketCount {
		return -1
	}
	return index
}

func singleValueOrEmpty(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	return ""
}

func scanRequest(scanner interface{ Scan(...any) error }) (model.RequestEvent, error) {
	var item model.RequestEvent
	var startedAt string
	var finishedAt string
	var success int64
	var aborted int64
	var stream int64

	if err := scanner.Scan(
		&item.RequestID,
		&item.ListenerName,
		&startedAt,
		&finishedAt,
		&item.Method,
		&item.Endpoint,
		&item.Model,
		&item.HTTPStatus,
		&success,
		&aborted,
		&stream,
		&item.ErrorMessage,
		&item.PromptTokens,
		&item.OutputTokens,
		&item.TotalTokens,
		&item.RequestDurationMs,
		&item.PromptEvalDurationMs,
		&item.EvalDurationMs,
		&item.UpstreamTotalDurationMs,
		&item.RequestSizeBytes,
		&item.ResponseSizeBytes,
		&item.ClientType,
		&item.ClientInstance,
		&item.AgentName,
		&item.SessionID,
		&item.RunID,
		&item.Workspace,
	); err != nil {
		return model.RequestEvent{}, err
	}

	item.StartedAt = parseTime(startedAt)
	item.FinishedAt = parseTime(finishedAt)
	item.Success = success == 1
	item.Aborted = aborted == 1
	item.Stream = stream == 1

	return item, nil
}

func scanSession(scanner interface{ Scan(...any) error }) (model.SessionSummary, error) {
	var item model.SessionSummary
	var firstSeen string
	var lastSeen string

	if err := scanner.Scan(
		&item.SessionID,
		&firstSeen,
		&lastSeen,
		&item.RequestCount,
		&item.PromptTokens,
		&item.OutputTokens,
		&item.TotalTokens,
		&item.ClientType,
		&item.ClientInstance,
		&item.AgentName,
		&item.Workspace,
	); err != nil {
		return model.SessionSummary{}, err
	}

	item.FirstSeenAt = parseTime(firstSeen)
	item.LastSeenAt = parseTime(lastSeen)
	return item, nil
}

func buildRequestWhere(filter model.RequestFilter) (string, []any) {
	var clauses []string
	var args []any

	if filter.Endpoint != "" {
		clauses = append(clauses, "endpoint = ?")
		args = append(args, filter.Endpoint)
	}
	if filter.Model != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.ClientType != "" {
		clauses = append(clauses, "client_type = ?")
		args = append(args, filter.ClientType)
	}
	if filter.ClientInstance != "" {
		clauses = append(clauses, "client_instance = ?")
		args = append(args, filter.ClientInstance)
	}
	if filter.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if filter.ListenerName != "" {
		clauses = append(clauses, "listener_name = ?")
		args = append(args, filter.ListenerName)
	}
	if !filter.StartedAfter.IsZero() {
		clauses = append(clauses, "started_at >= ?")
		args = append(args, formatTime(filter.StartedAfter))
	}
	if !filter.StartedBefore.IsZero() {
		clauses = append(clauses, "started_at < ?")
		args = append(args, formatTime(filter.StartedBefore))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
