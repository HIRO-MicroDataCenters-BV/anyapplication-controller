package types

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
)

type SyncResult struct {
	AggregatedStatus             *AggregatedStatus
	ApplicationResourcesDeployed bool
	ApplicationResourcesPresent  bool
	OperationPhaseStats          map[common.OperationPhase]int
	SyncPhaseStats               map[common.SyncPhase]int
	ResultCodeStats              map[common.ResultCode]int
	Total                        int
}

func NewSyncResult() *SyncResult {
	return &SyncResult{
		AggregatedStatus:             &AggregatedStatus{},
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

type DeleteResult struct {
	Version                     *SpecificVersion
	Total                       int
	Deleted                     int
	DeleteFailed                int
	ApplicationResourcesPresent bool
}

func IsApplicationResourcesPresent(results []*DeleteResult) bool {
	for _, result := range results {
		if result.ApplicationResourcesPresent {
			return true
		}
	}
	return false
}

type AggregatedStatus struct {
	HealthStatus *health.HealthStatus
	ChartVersion *SpecificVersion
}

type Applications interface {
	GetAllPresentVersions(application *v1.AnyApplication) (mapset.Set[*SpecificVersion], error)
	GetTargetVersion(application *v1.AnyApplication) mo.Option[*SpecificVersion]
	DetermineTargetVersion(application *v1.AnyApplication) (*SpecificVersion, error)

	GetInstanceId(application *v1.AnyApplication) string
	LoadApplication(application *v1.AnyApplication) (GlobalApplication, error)

	GetAggregatedStatusVersion(application *v1.AnyApplication, version *SpecificVersion) *AggregatedStatus
	SyncVersion(ctx context.Context, application *v1.AnyApplication, version *SpecificVersion) (*SyncResult, error)
	DeleteVersion(ctx context.Context, application *v1.AnyApplication, version *SpecificVersion) (*DeleteResult, error)
	Cleanup(ctx context.Context, application *v1.AnyApplication) ([]*DeleteResult, error)
}

type ResourceInfo struct {
	ManagedByMark string
}
