package global

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
)

type GlobalFSM struct {
	application        *v1.AnyApplication
	config             *config.ApplicationRuntimeConfig
	jobFactory         job.AsyncJobFactory
	applicationPresent bool
}

func NewGlobalFSM(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	applicationPresent bool,
) GlobalFSM {
	return GlobalFSM{
		application, config, jobFactory, applicationPresent,
	}
}

func (g *GlobalFSM) NextState() NextStateResult {
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

func (g *GlobalFSM) handlePlacementState() NextStateResult {
	status := g.application.Status
	spec := g.application.Spec

	if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
		condition, found := getCondition(&status, v1.PlacementConditionType, g.config.ZoneId)
		if !found {
			placementJob := g.jobFactory.CreateLocalPlacementJob(g.application)
			condition := placementJob.GetStatus()

			return NextStateResult{
				NextState:       mo.Some(v1.PlacementGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: NextJobs{
					JobsToAdd: mo.Some(placementJob),
				},
			}
		} else {
			if condition.Status == string(v1.PlacementStatusFailure) {
				return NextStateResult{
					NextState: mo.Some(v1.FailureGlobalState),
				}
			} else {
				return NextStateResult{
					NextState: mo.Some(v1.PlacementGlobalState),
				}
			}
		}
	}

	return NextStateResult{
		NextState: mo.Some(v1.PlacementGlobalState),
	}
}

func (g *GlobalFSM) handleOperationalState() NextStateResult {
	status := g.application.Status

	if isFailureCondition(g.application) {
		return g.handleFailureState()
	}

	if placementsContainZone(&status, g.config.ZoneId) {
		_, found := getCondition(&status, v1.LocalConditionType, g.config.ZoneId)

		if !found {
			operationJob := g.jobFactory.CreateOperationJob(g.application)
			condition := operationJob.GetStatus()

			return NextStateResult{
				NextState:       mo.Some(v1.OperationalGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: NextJobs{
					JobsToAdd: mo.Some(operationJob),
				},
			}
		}
	}
	if !placementsContainZone(&status, g.config.ZoneId) && g.applicationPresent {
		// Undeploy application
		operationCondition, _ := getCondition(&status, v1.LocalConditionType, g.config.ZoneId)

		condition, found := getCondition(&status, v1.RelocationConditionType, g.config.ZoneId)
		relocationCondition := mo.EmptyableToOption(condition)
		relocationJob := mo.None[job.AsyncJob]()
		if !found {
			undeployJob := g.jobFactory.CreateUndeployJob(g.application)
			cond := undeployJob.GetStatus()

			relocationCondition = mo.Some(&cond)
			relocationJob = mo.Some(undeployJob)
		}

		return NextStateResult{
			NextState:          mo.Some(v1.OperationalGlobalState),
			ConditionsToAdd:    relocationCondition,
			ConditionsToRemove: mo.EmptyableToOption(operationCondition),
			Jobs:               NextJobs{JobsToAdd: relocationJob},
		}

	}

	return NextStateResult{
		NextState: mo.Some(v1.OperationalGlobalState),
	}

}
func (g *GlobalFSM) handleRelocationState() NextStateResult {
	status := g.application.Status

	localRelocationCondition, found := getCondition(&status, v1.RelocationConditionType, g.config.ZoneId)
	if !found {
		relocationJob := g.jobFactory.CreateRelocationJob(g.application)
		condition := relocationJob.GetStatus()

		return NextStateResult{
			NextState:       mo.Some(v1.RelocationGlobalState),
			ConditionsToAdd: mo.Some(&condition),
			Jobs: NextJobs{
				JobsToAdd: mo.Some(relocationJob),
			},
		}
	} else {
		switch localRelocationCondition.Status {
		case string(v1.RelocationStatusDone):

			operationJob := g.jobFactory.CreateOperationJob(g.application)
			condition := operationJob.GetStatus()

			return NextStateResult{
				NextState:       mo.Some(v1.OperationalGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: NextJobs{
					JobsToAdd: mo.Some(operationJob),
				},
			}
		case string(v1.RelocationStatusFailure):

			relocationJob := g.jobFactory.CreateRelocationJob(g.application)
			condition := relocationJob.GetStatus()

			return NextStateResult{
				NextState:       mo.Some(v1.RelocationGlobalState),
				ConditionsToAdd: mo.Some(&condition),
				Jobs: NextJobs{
					JobsToAdd: mo.Some(relocationJob),
				},
			}

		case string(v1.RelocationStatusPull), string(v1.RelocationStatusUndeploy):
			return NextStateResult{
				NextState: mo.Some(v1.RelocationGlobalState),
			}
		default:
			panic("unexpected relocation status " + localRelocationCondition.Status)
		}
	}

}

func (g *GlobalFSM) handleFailureState() NextStateResult {
	// spec := g.application.Spec
	// if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
	// 	// TODO handle local state
	// }

	return NextStateResult{
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

func getCondition(status *v1.AnyApplicationStatus, conditionType v1.ApplicationConditionType, zoneId string) (*v1.ConditionStatus, bool) {
	condition, ok := lo.Find(status.Conditions, func(condition v1.ConditionStatus) bool {
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
