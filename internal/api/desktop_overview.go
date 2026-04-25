package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/trevorashby/llamasitter/internal/model"
)

func (s *Server) handleDesktopOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	overview, err := desktopOverview(r.Context(), s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

func desktopOverview(ctx context.Context, store QueryStore) (*model.DesktopOverview, error) {
	summary, err := store.UsageSummary(ctx, model.RequestFilter{})
	if err != nil {
		return nil, fmt.Errorf("usage summary: %w", err)
	}

	sessions, err := store.ListSessions(ctx, model.RequestFilter{Limit: 5})
	if err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}

	requests, err := store.ListRequests(ctx, model.RequestFilter{Limit: 5})
	if err != nil {
		return nil, fmt.Errorf("requests: %w", err)
	}

	activityTitle := "No activity yet"
	activityDetail := "Requests and sessions will appear here once the proxy captures traffic."

	if len(sessions) > 0 {
		session := sessions[0]
		activityTitle = fmt.Sprintf("Session %s", defaultString(session.SessionID, "Recent session"))
		if session.AgentName != "" {
			activityDetail = fmt.Sprintf("%d requests • %d tokens • %s",
				session.RequestCount,
				session.TotalTokens,
				session.AgentName,
			)
		} else {
			activityDetail = fmt.Sprintf("%d requests • %d tokens",
				session.RequestCount,
				session.TotalTokens,
			)
		}
	} else if len(requests) > 0 {
		request := requests[0]
		activityTitle = defaultString(request.Model, "Recent request")
		activityDetail = fmt.Sprintf("HTTP %d • %d tokens", request.HTTPStatus, request.TotalTokens)
	}

	return &model.DesktopOverview{
		LastRefreshAt:            time.Now().UTC(),
		RequestCount:             summary.RequestCount,
		TotalTokens:              summary.TotalTokens,
		AverageRequestDurationMs: summary.AvgRequestDurationMs,
		TopModel:                 breakdownKey(summary.ByModel, "No data yet"),
		TopClientInstance:        breakdownKey(summary.ByClientInstance, "No data yet"),
		ActivityTitle:            activityTitle,
		ActivityDetail:           activityDetail,
	}, nil
}

func breakdownKey(rows []model.BreakdownRow, fallback string) string {
	if len(rows) == 0 || rows[0].Key == "" {
		return fallback
	}
	return rows[0].Key
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
