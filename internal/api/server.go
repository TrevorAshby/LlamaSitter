package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trevorashby/llamasitter/internal/analytics"
	"github.com/trevorashby/llamasitter/internal/model"
	"github.com/trevorashby/llamasitter/internal/storage"
	webassets "github.com/trevorashby/llamasitter/web"
)

type QueryStore interface {
	storage.Store
}

type Server struct {
	store  QueryStore
	logger *slog.Logger
	assets http.Handler
}

func NewServer(addr string, store QueryStore, logger *slog.Logger) *http.Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	sub, _ := fs.Sub(webassets.StaticFS, "static")
	server := &Server{
		store:  store,
		logger: logger,
		assets: http.FileServer(http.FS(sub)),
	}

	return &http.Server{
		Addr:              addr,
		Handler:           server.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/api/requests", s.handleRequests)
	mux.HandleFunc("/api/requests/", s.handleRequestByID)
	mux.HandleFunc("/api/usage/summary", s.handleUsageSummary)
	mux.HandleFunc("/api/usage/timeseries", s.handleUsageTimeseries)
	mux.HandleFunc("/api/usage/heatmap", s.handleUsageHeatmap)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByID)
	mux.HandleFunc("/api/export/requests.json", s.handleExportJSON)
	mux.HandleFunc("/api/export/requests.csv", s.handleExportCSV)
	mux.Handle("/", s.assets)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter, err := requestFilterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := s.store.ListRequests(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ListResponse[model.RequestEvent]{
		Items: items,
		Count: len(items),
	})
}

func (s *Server) handleRequestByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing request id")
		return
	}

	item, err := s.store.GetRequest(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "request not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter, err := requestFilterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	summary, err := s.store.UsageSummary(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleUsageTimeseries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rangeName := strings.TrimSpace(r.URL.Query().Get("range"))
	if rangeName == "" {
		rangeName = "day"
	}

	filter, err := requestFilterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.StartedAfter, filter.StartedBefore = normalizeWindowFromFilter(rangeName, filter.StartedAfter, filter.StartedBefore, time.Now().UTC())

	items, err := s.store.UsageTimeseries(r.Context(), filter, rangeName, queryBool(r, "include_breakdowns"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ListResponse[model.TimeBucket]{
		Items: items,
		Count: len(items),
	})
}

func (s *Server) handleUsageHeatmap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rangeName := strings.TrimSpace(r.URL.Query().Get("range"))
	if rangeName == "" {
		rangeName = "week"
	}

	filter, err := requestFilterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.StartedAfter, filter.StartedBefore = normalizeWindowFromFilter(rangeName, filter.StartedAfter, filter.StartedBefore, time.Now().UTC())

	items, err := s.store.UsageHeatmap(r.Context(), filter, queryInt(r, "tz_offset_minutes", 0), queryBool(r, "include_breakdowns"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ListResponse[model.HeatmapCell]{
		Items: items,
		Count: len(items),
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter, err := requestFilterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !hasQueryValue(r, "limit") {
		filter.Limit = 50
	}

	items, err := s.store.ListSessions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ListResponse[model.SessionSummary]{
		Items: items,
		Count: len(items),
	})
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	session, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	requests, err := s.store.ListRequests(r.Context(), model.RequestFilter{
		SessionID: id,
		Limit:     100,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.SessionDetail{
		Session:  *session,
		Requests: requests,
	})
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.store.ListRequests(r.Context(), model.RequestFilter{
		Limit: 0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="requests.json"`)
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.store.ListRequests(r.Context(), model.RequestFilter{
		Limit: 0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="requests.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{
		"request_id",
		"started_at",
		"finished_at",
		"listener_name",
		"endpoint",
		"model",
		"http_status",
		"success",
		"aborted",
		"stream",
		"prompt_tokens",
		"output_tokens",
		"total_tokens",
		"request_duration_ms",
		"client_type",
		"client_instance",
		"agent_name",
		"session_id",
		"run_id",
		"workspace",
	})

	for _, item := range items {
		_ = writer.Write([]string{
			item.RequestID,
			item.StartedAt.Format(time.RFC3339),
			item.FinishedAt.Format(time.RFC3339),
			item.ListenerName,
			item.Endpoint,
			item.Model,
			strconv.Itoa(item.HTTPStatus),
			strconv.FormatBool(item.Success),
			strconv.FormatBool(item.Aborted),
			strconv.FormatBool(item.Stream),
			strconv.FormatInt(item.PromptTokens, 10),
			strconv.FormatInt(item.OutputTokens, 10),
			strconv.FormatInt(item.TotalTokens, 10),
			strconv.FormatInt(item.RequestDurationMs, 10),
			item.ClientType,
			item.ClientInstance,
			item.AgentName,
			item.SessionID,
			item.RunID,
			item.Workspace,
		})
	}
}

func requestFilterFromQuery(r *http.Request) (model.RequestFilter, error) {
	filter := model.RequestFilter{
		Limit:          queryInt(r, "limit", 100),
		Offset:         queryInt(r, "offset", 0),
		Endpoint:       r.URL.Query().Get("endpoint"),
		Model:          r.URL.Query().Get("model"),
		ClientType:     r.URL.Query().Get("client_type"),
		ClientInstance: r.URL.Query().Get("client_instance"),
		SessionID:      r.URL.Query().Get("session_id"),
		ListenerName:   r.URL.Query().Get("listener_name"),
	}

	startedAfter, err := queryTime(r, "started_after")
	if err != nil {
		return model.RequestFilter{}, err
	}
	startedBefore, err := queryTime(r, "started_before")
	if err != nil {
		return model.RequestFilter{}, err
	}
	if !startedAfter.IsZero() && !startedBefore.IsZero() && !startedAfter.Before(startedBefore) {
		return model.RequestFilter{}, errors.New("started_after must be before started_before")
	}

	filter.StartedAfter = startedAfter
	filter.StartedBefore = startedBefore
	return filter, nil
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryBool(r *http.Request, key string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false
	}

	value, err := strconv.ParseBool(raw)
	if err == nil {
		return value
	}

	switch strings.ToLower(raw) {
	case "yes", "y", "on":
		return true
	default:
		return false
	}
}

func queryTime(r *http.Request, key string) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return time.Time{}, nil
	}

	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s", key)
	}
	return value.UTC(), nil
}

func normalizeWindowFromFilter(rangeName string, startedAfter, startedBefore, now time.Time) (time.Time, time.Time) {
	return analytics.NormalizeWindow(rangeName, now, startedAfter, startedBefore)
}

func hasQueryValue(r *http.Request, key string) bool {
	_, ok := r.URL.Query()[key]
	return ok
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func Serve(ctx context.Context, srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
