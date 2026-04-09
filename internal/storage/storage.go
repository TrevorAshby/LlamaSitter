package storage

import (
	"context"

	"github.com/trevorashby/llamasitter/internal/model"
)

type Store interface {
	Close() error
	Ping(context.Context) error
	Migrate(context.Context) error
	InsertRequest(context.Context, *model.RequestEvent) error
	ListRequests(context.Context, model.RequestFilter) ([]model.RequestEvent, error)
	GetRequest(context.Context, string) (*model.RequestEvent, error)
	UsageSummary(context.Context, model.RequestFilter) (*model.UsageSummary, error)
	UsageTimeseries(context.Context, model.RequestFilter, string, bool) ([]model.TimeBucket, error)
	UsageHeatmap(context.Context, model.RequestFilter, int, bool) ([]model.HeatmapCell, error)
	ListSessions(context.Context, model.RequestFilter) ([]model.SessionSummary, error)
	GetSession(context.Context, string) (*model.SessionSummary, error)
}
