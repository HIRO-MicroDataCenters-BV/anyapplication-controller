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
	ApplicationResourcesPresent  bool
	OperationPhaseStats          map[common.OperationPhase]int
	SyncPhaseStats               map[common.SyncPhase]int
	ResultCodeStats              map[common.ResultCode]int
	Total                        int
}

type DeleteResult struct {
	Total                       int
	Deleted                     int
	DeleteFailed                int
	ApplicationResourcesPresent bool
}

func NewSyncResult() *SyncResult {
	return &SyncResult{
		Status:                       &health.HealthStatus{},
		OperationPhaseStats:          make(map[common.OperationPhase]int),
		SyncPhaseStats:               make(map[common.SyncPhase]int),
		ResultCodeStats:              make(map[common.ResultCode]int),
		ApplicationResourcesDeployed: false,
		ApplicationResourcesPresent:  false,
	}
}

func (s *SyncResult) AddResult(r *common.ResourceSyncResult) {
	s.Total += 1
	s.OperationPhaseStats[r.HookPhase] += 1
	s.SyncPhaseStats[r.SyncPhase] += 1
	s.ResultCodeStats[r.Status] += 1
}

type Applications interface {
	GetAggregatedStatus(application *v1.AnyApplication) *health.HealthStatus
	GetAggregatedStatusVersion(application *v1.AnyApplication, version *SpecificVersion) *health.HealthStatus
	Sync(ctx context.Context, application *v1.AnyApplication) (*SyncResult, error)
	Delete(ctx context.Context, application *v1.AnyApplication) (*DeleteResult, error)
	GetInstanceId(application *v1.AnyApplication) string
	LoadApplication(application *v1.AnyApplication) (GlobalApplication, error)

	SyncVersion(ctx context.Context, application *v1.AnyApplication, version *SpecificVersion) (*SyncResult, error)
	DeleteVersion(ctx context.Context, application *v1.AnyApplication, version *SpecificVersion) (*DeleteResult, error)
	DetermineTargetVersion(application *v1.AnyApplication) (*SpecificVersion, error)
}

type ResourceInfo struct {
	ManagedByMark string
}
