package global

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/types"
)

type GlobalFSM struct {
	application        *v1.AnyApplication
	config             *config.ApplicationRuntimeConfig
	jobFactory         types.AsyncJobFactory
	applicationPresent bool
	runningJobType     mo.Option[types.AsyncJobType]
}

func NewGlobalFSM(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	runningJobType mo.Option[types.AsyncJobType],
) GlobalFSM {
	return GlobalFSM{
		application, config, jobFactory, applicationPresent, runningJobType,
	}
}

func (g *GlobalFSM) isRunning(jobType types.AsyncJobType) bool {
	return g.runningJobType.OrEmpty() == jobType
}

func (g *GlobalFSM) NextState() types.NextStateResult {
	status := &g.application.Status

	if !placementExists(status) {
		return g.handlePlacementState()
	}
	if isFailureCondition(g.application) {
		return g.handleFailureState()
	}
	state := getGlobalState(&g.application.Status)
	return types.NextStateResult{
		NextState: mo.Some(state),
	}
}

func (g *GlobalFSM) handlePlacementState() types.NextStateResult {
	spec := g.application.Spec
	status := g.application.Status

	zoneStatus, zoneStatusFound := status.GetStatusFor(g.config.ZoneId)
	conditions := make([]v1.ConditionStatus, 0)
	if zoneStatusFound {
		conditions = zoneStatus.Conditions
	}

	if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
		condition, found := getCondition(conditions, v1.PlacementConditionType, g.config.ZoneId)
		if !found || !g.isRunning(types.AsyncJobTypeLocalPlacement) {
			placementJob := g.jobFactory.CreateLocalPlacementJob(g.application)
			condition := placementJob.GetStatus()

			return types.NextStateResult{
				NextState:       mo.Some(v1.PlacementGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs:            types.NextJobs{JobsToAdd: mo.Some(placementJob)},
			}
		} else {
			if condition.Status == string(v1.PlacementStatusFailure) {
				return types.NextStateResult{
					NextState: mo.Some(v1.FailureGlobalState),
				}
			} else {
				return types.NextStateResult{
					NextState: mo.Some(v1.PlacementGlobalState),
				}
			}
		}
	}

	return types.NextStateResult{
		NextState: mo.Some(v1.PlacementGlobalState),
	}
}

func (g *GlobalFSM) handleFailureState() types.NextStateResult {
	return types.NextStateResult{
		NextState: mo.Some(v1.FailureGlobalState),
	}
}

func getGlobalState(status *v1.AnyApplicationStatus) v1.GlobalState {
	state := v1.OperationalGlobalState
	// TODO state ordering
	for _, placement := range status.Placements {
		zoneStatus, zoneStatusFound := status.GetStatusFor(placement.Zone)
		if !zoneStatusFound {
			state = v1.RelocationGlobalState
		} else {
			conditionOpt := getHighestZoneCondition(zoneStatus, placement.Zone)
			condition, present := conditionOpt.Get()
			if present && (condition.Type == v1.DeploymentConditionType || condition.Type == v1.UndeploymentConditionType) {
				state = v1.RelocationGlobalState
			} else if !present {
				state = v1.RelocationGlobalState
			}
		}
	}
	return state
}

func getHighestZoneCondition(zoneStatus *v1.ZoneStatus, zoneId string) mo.Option[*v1.ConditionStatus] {
	conditionTypes := []v1.ApplicationConditionType{
		v1.LocalConditionType,
		v1.DeploymentConditionType,
		v1.UndeploymentConditionType,
	}
	for _, conditionType := range conditionTypes {
		condition, found := getCondition(zoneStatus.Conditions, conditionType, zoneId)
		if found {
			return mo.Some(condition)
		}
	}
	return mo.None[*v1.ConditionStatus]()
}

func getCondition(conditions []v1.ConditionStatus, conditionType v1.ApplicationConditionType, zoneId string) (*v1.ConditionStatus, bool) {
	condition, ok := lo.Find(conditions, func(condition v1.ConditionStatus) bool {
		return condition.Type == conditionType && condition.ZoneId == zoneId
	})
	return &condition, ok
}

func addOrUpdateCondition(status *v1.AnyApplicationStatus, condition *v1.ConditionStatus, zoneId string) {
	zoneStatus := status.GetOrCreateStatusFor(zoneId)
	existing, ok := lo.Find(zoneStatus.Conditions, func(existing v1.ConditionStatus) bool {
		return existing.Type == condition.Type && existing.ZoneId == condition.ZoneId
	})
	if !ok {
		zoneStatus.Conditions = append(zoneStatus.Conditions, *condition)
	} else {
		condition.DeepCopyInto(&existing)
	}
}

func removeCondition(status *v1.AnyApplicationStatus, toRemove *v1.ConditionStatus, zoneId string) {
	zoneStatus, exists := status.GetStatusFor(zoneId)
	if !exists {
		return
	}
	zoneStatus.Conditions = lo.Filter(zoneStatus.Conditions, func(existing v1.ConditionStatus, _ int) bool {
		equal := existing.Type == toRemove.Type && existing.ZoneId == toRemove.ZoneId
		return !equal
	})
}

func setNewVersion(status *v1.AnyApplicationStatus, newVersion *types.SpecificVersion, zoneId string) {
	zoneStatus := status.GetOrCreateStatusFor(zoneId)
	zoneStatus.ChartVersion = newVersion.ToString()
}

func isFailureCondition(application *v1.AnyApplication) bool {
	status := &application.Status
	spec := &application.Spec

	failedConditionCount := 0
	for _, zoneStatus := range status.Zones {
		zoneFailedConditions := false
		for _, condition := range zoneStatus.Conditions {
			switch condition.Type {
			case v1.LocalConditionType:
				if condition.Status == string(health.HealthStatusDegraded) || condition.Status == string(health.HealthStatusMissing) {
					zoneFailedConditions = true
				}
			case v1.DeploymentConditionType:
				if condition.Status == string(v1.DeploymentStatusFailure) {
					zoneFailedConditions = true
				}
			case v1.UndeploymentConditionType:
				if condition.Status == string(v1.UndeploymentStatusFailure) {
					zoneFailedConditions = true
				}
			}
		}
		if zoneFailedConditions {
			failedConditionCount++
		}
	}
	return failedConditionCount > spec.RecoverStrategy.Tolerance
}

func placementExists(status *v1.AnyApplicationStatus) bool {
	return status.Placements != nil
}
func placementsContainZone(status *v1.AnyApplicationStatus, currentZone string) bool {
	if status.Placements == nil {
		return false
	}
	_, ok := lo.Find(status.Placements, func(placement v1.Placement) bool {
		return placement.Zone == currentZone
	})
	return ok
}
