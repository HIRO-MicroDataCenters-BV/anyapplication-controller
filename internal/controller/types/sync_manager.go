package types

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	v1 "hiro.io/anyapplication/api/v1"
)

type SyncResult struct {
	Status       *health.HealthStatus
	Created      int
	CreateFailed int
	Updated      int
	UpdateFailed int
	Deleted      int
	DeleteFailed int
	Total        int
}

type SyncManager interface {
	Sync(ctx context.Context, application *v1.AnyApplication) (SyncResult, error)
	Delete(ctx context.Context, application *v1.AnyApplication) (SyncResult, error)
	LoadApplication(application *v1.AnyApplication) (GlobalApplication, error)
	InvalidateCache() error
}
