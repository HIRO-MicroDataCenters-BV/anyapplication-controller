package sync

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	v1 "hiro.io/anyapplication/api/v1"
)

type SyncManager interface {
	Sync(ctx context.Context, application *v1.AnyApplication) (*health.HealthStatus, error)
	Delete(ctx context.Context, application *v1.AnyApplication) error
	InvalidateCache() error
}
