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

func (g *GlobalFSM) NextState() types.NextStateResult {
	status := g.application.Status

	if !placementExists(&status) {
		return g.handlePlacementState()
	}
	if operationalInAllPlacementZones(&status) {
		return g.handleOperationalState()
	} else {
		return g.handleRelocationState()
	}
}

func (g *GlobalFSM) handlePlacementState() types.NextStateResult {
	status := g.application.Status
	spec := g.application.Spec

	if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
		condition, found := getCondition(status.Conditions, v1.PlacementConditionType, g.config.ZoneId)
		if !found || !g.isRunning(types.AsyncJobTypeLocalPlacement) {
			placementJob := g.jobFactory.CreateLocalPlacementJob(g.application)
			condition := placementJob.GetStatus()

			return types.NextStateResult{
				NextState:       mo.Some(v1.PlacementGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: types.NextJobs{
					JobsToAdd: mo.Some(placementJob),
				},
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

func (g *GlobalFSM) isRunning(jobType types.AsyncJobType) bool {
	return g.runningJobType.OrEmpty() == jobType
}

func (g *GlobalFSM) handleOperationalState() types.NextStateResult {
	status := g.application.Status

	if isFailureCondition(g.application) {
		return g.handleFailureState()
	}

	if placementsContainZone(&status, g.config.ZoneId) {
		_, found := getCondition(status.Conditions, v1.LocalConditionType, g.config.ZoneId)

		if !found || !g.isRunning(types.AsyncJobTypeLocalOperation) {
			operationJob := g.jobFactory.CreateOperationJob(g.application)
			condition := operationJob.GetStatus()

			return types.NextStateResult{
				NextState:       mo.Some(v1.OperationalGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: types.NextJobs{
					JobsToAdd: mo.Some(operationJob),
				},
			}
		}
	}
	if !placementsContainZone(&status, g.config.ZoneId) && g.applicationPresent {
		// Undeploy application
		operationCondition, _ := getCondition(status.Conditions, v1.LocalConditionType, g.config.ZoneId)

		condition, found := getCondition(status.Conditions, v1.RelocationConditionType, g.config.ZoneId)
		relocationCondition := mo.EmptyableToOption(condition)
		relocationJob := mo.None[types.AsyncJob]()
		if !found {
			undeployJob := g.jobFactory.CreateUndeployJob(g.application)
			cond := undeployJob.GetStatus()

			relocationCondition = mo.Some(&cond)
			relocationJob = mo.Some(undeployJob)
		}

		return types.NextStateResult{
			NextState:          mo.Some(v1.OperationalGlobalState),
			ConditionsToAdd:    relocationCondition,
			ConditionsToRemove: mo.EmptyableToOption(operationCondition),
			Jobs:               types.NextJobs{JobsToAdd: relocationJob},
		}

	}

	return types.NextStateResult{
		NextState: mo.Some(v1.OperationalGlobalState),
	}

}
func (g *GlobalFSM) handleRelocationState() types.NextStateResult {
	status := g.application.Status

	localRelocationCondition, found := getCondition(status.Conditions, v1.RelocationConditionType, g.config.ZoneId)
	if !found || !g.isRunning(types.AsyncJobTypeRelocate) {
		relocationJob := g.jobFactory.CreateRelocationJob(g.application)
		condition := relocationJob.GetStatus()

		return types.NextStateResult{
			NextState:       mo.Some(v1.RelocationGlobalState),
			ConditionsToAdd: mo.Some(&condition),
			Jobs: types.NextJobs{
				JobsToAdd: mo.Some(relocationJob),
			},
		}
	} else {
		switch localRelocationCondition.Status {
		case string(v1.RelocationStatusDone):

			operationJob := g.jobFactory.CreateOperationJob(g.application)
			condition := operationJob.GetStatus()

			return types.NextStateResult{
				NextState:       mo.Some(v1.OperationalGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: types.NextJobs{
					JobsToAdd: mo.Some(operationJob),
				},
			}
		case string(v1.RelocationStatusFailure):

			relocationJob := g.jobFactory.CreateRelocationJob(g.application)
			condition := relocationJob.GetStatus()

			return types.NextStateResult{
				NextState:       mo.Some(v1.RelocationGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: types.NextJobs{
					JobsToAdd: mo.Some(relocationJob),
				},
			}

		case string(v1.RelocationStatusPull), string(v1.RelocationStatusUndeploy):
			return types.NextStateResult{
				NextState: mo.Some(v1.RelocationGlobalState),
			}
		default:
			panic("unexpected relocation status " + localRelocationCondition.Status)
		}
	}

}

func (g *GlobalFSM) handleFailureState() types.NextStateResult {
	// spec := g.application.Spec
	// if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
	// 	// TODO handle local state
	// }

	return types.NextStateResult{
		NextState: mo.Some(v1.FailureGlobalState),
	}
}

func operationalInAllPlacementZones(status *v1.AnyApplicationStatus) bool {
	for _, placement := range status.Placements {
		if !conditionExists(status, v1.LocalConditionType, placement.Zone) {
			return false
		}
	}
	return true
}

func conditionExists(status *v1.AnyApplicationStatus, conditionType v1.ApplicationConditionType, zoneId string) bool {
	_, ok := lo.Find(status.Conditions, func(condition v1.ConditionStatus) bool {
		return condition.Type == conditionType && condition.ZoneId == zoneId
	})
	return ok
}

func getCondition(conditions []v1.ConditionStatus, conditionType v1.ApplicationConditionType, zoneId string) (*v1.ConditionStatus, bool) {
	condition, ok := lo.Find(conditions, func(condition v1.ConditionStatus) bool {
		return condition.Type == conditionType && condition.ZoneId == zoneId
	})
	return &condition, ok
}

func addOrUpdateCondition(status *v1.AnyApplicationStatus, condition *v1.ConditionStatus) {
	existing, ok := lo.Find(status.Conditions, func(existing v1.ConditionStatus) bool {
		return existing.Type == condition.Type && existing.ZoneId == condition.ZoneId
	})
	if !ok {
		status.Conditions = append(status.Conditions, *condition)
	} else {
		condition.DeepCopyInto(&existing)
	}
}

func removeCondition(status *v1.AnyApplicationStatus, toRemove *v1.ConditionStatus) {
	status.Conditions = lo.Filter(status.Conditions, func(existing v1.ConditionStatus, _ int) bool {
		equal := existing.Type == toRemove.Type && existing.ZoneId == toRemove.ZoneId
		return !equal
	})
}

func isFailureCondition(application *v1.AnyApplication) bool {
	status := &application.Status
	spec := &application.Spec

	failedConditions := 0
	for _, condition := range status.Conditions {
		if condition.Type == v1.LocalConditionType {
			if condition.Status == string(health.HealthStatusDegraded) || condition.Status == string(health.HealthStatusMissing) {
				failedConditions++
			}
		}
	}
	return failedConditions > spec.RecoverStrategy.Tolerance
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
