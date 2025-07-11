package types

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	v1 "hiro.io/anyapplication/api/v1"
)

type SyncResult struct {
	Status                       *health.HealthStatus
	ApplicationResourcesDeployed bool
	OperationPhaseStats          map[common.OperationPhase]int
	SyncPhaseStats               map[common.SyncPhase]int
	ResultCodeStats              map[common.ResultCode]int
	Total                        int
}

type DeleteResult struct {
	Total        int
	Deleted      int
	DeleteFailed int
}

func NewSyncResult() *SyncResult {
	return &SyncResult{
		Status:                       &health.HealthStatus{},
		OperationPhaseStats:          make(map[common.OperationPhase]int),
		SyncPhaseStats:               make(map[common.SyncPhase]int),
		ResultCodeStats:              make(map[common.ResultCode]int),
		ApplicationResourcesDeployed: false,
	}
}

func (s *SyncResult) AddResult(r *common.ResourceSyncResult) {
	s.Total += 1
	s.OperationPhaseStats[r.HookPhase] += 1
	s.SyncPhaseStats[r.SyncPhase] += 1
	s.ResultCodeStats[r.Status] += 1
}

type SyncManager interface {
	GetAggregatedStatus(application *v1.AnyApplication) *health.HealthStatus
	Sync(ctx context.Context, application *v1.AnyApplication) (*SyncResult, error)
	Delete(ctx context.Context, application *v1.AnyApplication) (*DeleteResult, error)
	LoadApplication(application *v1.AnyApplication) (GlobalApplication, error)
}

type ResourceInfo struct {
	ManagedByMark string
}
