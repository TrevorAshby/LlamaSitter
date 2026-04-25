package model

import "time"

type DesktopOverview struct {
	LastRefreshAt            time.Time `json:"last_refresh_at"`
	RequestCount             int64     `json:"request_count"`
	TotalTokens              int64     `json:"total_tokens"`
	AverageRequestDurationMs float64   `json:"average_request_duration_ms"`
	TopModel                 string    `json:"top_model"`
	TopClientInstance        string    `json:"top_client_instance"`
	ActivityTitle            string    `json:"activity_title"`
	ActivityDetail           string    `json:"activity_detail"`
}
