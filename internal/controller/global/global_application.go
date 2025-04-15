package global

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/moutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action string

const (
	RelocateToCurrentNode Action = "RelocateToCurrentNode"
)

type GlobalApplication struct {
	locaApplication mo.Option[local.LocalApplication]
	application     *v1.AnyApplication
	config          *config.ApplicationRuntimeConfig
}

func LoadCurrentState(
	ctx context.Context,
	client client.Client,
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
) (GlobalApplication, error) {
	localApplication, err := local.LoadCurrentState(ctx, client, &application.Spec.Application, config)
	if err != nil {
		return GlobalApplication{}, errors.Wrap(err, "Failed to load local application")
	}
	return NewFromLocalApplication(localApplication, application, config), nil
}

func NewFromLocalApplication(
	localApplication mo.Option[local.LocalApplication],
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
) GlobalApplication {
	return GlobalApplication{
		locaApplication: localApplication,
		application:     application,
		config:          config,
	}
}

func (g *GlobalApplication) DeriveNewStatus(
	jobConditions JobApplicationConditions,
	jobFactory job.AsyncJobFactory,
) mo.Option[v1.AnyApplicationStatus] {

	status := g.application.Status

	// Update local application status if exists
	localConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
		return l.GetCondition()
	})
	stateUpdated := moutils.Map(localConditionOpt, func(condition v1.ConditionStatus) bool {
		updateLocalCondition(&status, &condition, g.config)
		return true
	}).OrElse(false)

	// Update loca job conditions
	stateUpdated = updateJobConditions(&status, jobConditions) || stateUpdated

	// Update global state
	globalStateUpdated := updateGlobalState(g.application, g.config, jobFactory)
	stateUpdated = globalStateUpdated || stateUpdated

	if stateUpdated {
		return mo.Some(status)
	} else {
		return mo.None[v1.AnyApplicationStatus]()
	}
}

func updateLocalCondition(status *v1.AnyApplicationStatus, condition *v1.ConditionStatus, config *config.ApplicationRuntimeConfig) {
	found, ok := lo.Find(status.Conditions, func(cond v1.ConditionStatus) bool {
		return cond.ZoneId == config.ZoneId
	})
	if !ok {
		status.Conditions = append(status.Conditions, *condition)
	} else {
		condition.DeepCopyInto(&found)
	}
}

func updateGlobalState(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
) bool {
	status := &application.Status

	// Not owner cannot update the state
	if status.Owner != config.ZoneId {
		return false
	}

	stateUpdated := false
	nextStateResult := nextState(application, config, jobFactory)
	maybeNextState, conditionsToAdd, conditionsToRemove := nextStateResult.nextState, nextStateResult.conditionsToAdd, nextStateResult.conditionsToRemove

	conditionsToRemove.ForEach(func(condition *v1.ConditionStatus) {
		removeCondition(status, condition)
		stateUpdated = true
	})

	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(status, condition)
		stateUpdated = true
	})

	nextState := maybeNextState.OrElse(status.State)
	if status.State != nextState {
		status.State = nextState
		stateUpdated = true
	}

	return stateUpdated
}

type nextStateResult struct {
	nextState          mo.Option[v1.GlobalState]
	conditionsToAdd    mo.Option[*v1.ConditionStatus]
	conditionsToRemove mo.Option[*v1.ConditionStatus]
	jobsToAdd          mo.Option[job.AsyncJob]
	jobsToRemove       mo.Option[job.AsyncJobType]
}

func nextState(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
) nextStateResult {
	spec := &application.Spec
	status := &application.Status

	if status.State == v1.NewGlobalState {
		// if current state is new and current node is owner
		return nextStateResult{
			nextState: mo.Some(v1.PlacementGlobalState),
		}
	} else if status.State == v1.PlacementGlobalState {
		// if current state is Placement and current node is owner

		if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
			// Local Placement strategy
			if !conditionExists(status, v1.PlacementConditionType, config.ZoneId) {
				placement := v1.Placement{
					Zone: config.ZoneId,
				}
				status.Placements = append(status.Placements, placement)
				condition := NewLocalPlacementCondition(config.ZoneId)
				return nextStateResult{
					nextState:       mo.Some(v1.OperationalGlobalState),
					conditionsToAdd: mo.Some(&condition),
				}
			}
		} else {
			if len(status.Placements) == 0 {
				// Wait for global placement strategy to decide about placement
				return nextStateResult{}
			}
			// Global Placement strategy
			if !placementsContainZone(status, config.ZoneId) {
				// Current node is owner but not in the placement list
				return nextStateResult{
					nextState: mo.Some(v1.OwnershipTransferGlobalState),
				}
			} else {
				// Transit to operational state
				return nextStateResult{
					nextState: mo.Some(v1.OperationalGlobalState),
				}
			}

		}
	} else if status.State == v1.OperationalGlobalState {
		if !placementsContainZone(status, config.ZoneId) {
			condition := mo.None[*v1.ConditionStatus]()
			job := mo.None[job.AsyncJob]()
			if !conditionExists(status, v1.PlacementConditionType, config.ZoneId) {
				transferJob := jobFactory.CreateOnwershipTransferJob(application)
				cond := transferJob.GetStatus()
				condition = mo.Some(&cond)
			}
			return nextStateResult{
				nextState:       mo.Some(v1.OwnershipTransferGlobalState),
				conditionsToAdd: condition,
				jobsToAdd:       job,
			}
		} else {
			_, foundLocal := getCondition(status, v1.LocalConditionType, config.ZoneId)
			if !foundLocal {
				// TODO return new relocation job
				relocationCondition := mo.None[*v1.ConditionStatus]()
				if !conditionExists(status, v1.PlacementConditionType, config.ZoneId) {
					cond := NewOwnershipTransferCondition(config.ZoneId)
					relocationCondition = mo.Some(&cond)
				}
				return nextStateResult{
					nextState:       mo.Some(v1.RelocationGlobalState),
					conditionsToAdd: relocationCondition,
				}
			}

			if isFailureCondition(application) {
				return nextStateResult{
					nextState: mo.Some(v1.FailureGlobalState),
				}
			}
		}
	} else if status.State == v1.FailureGlobalState {
		if !isFailureCondition(application) {
			return nextStateResult{
				nextState: mo.Some(v1.OperationalGlobalState),
			}
		}
	} else if status.State == v1.RelocationGlobalState {
		condition, ok := getCondition(status, v1.RelocationConditionType, config.ZoneId)
		if !ok {
			relocationJob := jobFactory.CreateRelocationJob(application)
			condition := relocationJob.GetStatus()
			return nextStateResult{
				nextState:       mo.Some(v1.RelocationGlobalState),
				conditionsToAdd: mo.Some(&condition),
				jobsToAdd:       mo.Some(relocationJob),
			}
		}
		if condition.Status == string(v1.RelocationStatusDone) {
			return nextStateResult{
				nextState:          mo.Some(v1.OperationalGlobalState),
				conditionsToRemove: mo.Some(condition),
				jobsToRemove:       mo.Some(job.AsyncJobTypeRelocate),
			}
		}
		if condition.Status == string(v1.RelocationStatusFailure) {
			relocationJob := jobFactory.CreateRelocationJob(application) // Add attempt counter
			condition := relocationJob.GetStatus()
			return nextStateResult{
				nextState:       mo.Some(v1.OperationalGlobalState),
				jobsToAdd:       mo.Some(relocationJob),
				conditionsToAdd: mo.Some(&condition),
			}
		}

	} else if status.State == v1.OwnershipTransferGlobalState {
		if placementsContainZone(status, config.ZoneId) {
			condition, found := getCondition(status, v1.LocalConditionType, config.ZoneId)
			conditionToRemove := mo.None[*v1.ConditionStatus]()
			if found {
				if condition.Status == string(v1.OwnershipTransferSuccess) {
					conditionToRemove = mo.Some(condition)
				} else {
					// TODO failure case
				}
			}
			return nextStateResult{
				nextState:          mo.Some(v1.OperationalGlobalState),
				conditionsToRemove: conditionToRemove,
				jobsToRemove:       mo.Some(job.AsyncJobTypeOwnershipTransfer),
			}
		} else {
			condition, found := getCondition(status, v1.LocalConditionType, config.ZoneId)

			if condition.Status == string(v1.OwnershipTransferFailure) {
				// TODO failure case
			}
			var conditionToAdd mo.Option[*v1.ConditionStatus]
			transferJob := mo.None[job.AsyncJob]()
			if !found {
				job := jobFactory.CreateRelocationJob(application)
				jobStatus := job.GetStatus()
				conditionToAdd = mo.Some(&jobStatus)
				transferJob = mo.Some(job)
			}
			return nextStateResult{
				nextState:       mo.Some(v1.OwnershipTransferGlobalState),
				conditionsToAdd: conditionToAdd,
				jobsToAdd:       transferJob,
			}
		}
	}

	return nextStateResult{}
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

func updateJobConditions(status *v1.AnyApplicationStatus, jobConditions JobApplicationConditions) bool {
	stateUpdated := false
	for _, condition := range jobConditions.Conditions {
		addOrUpdateCondition(status, condition)
		stateUpdated = true
	}
	return stateUpdated
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

func NewLocalPlacementCondition(zoneId string) v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.PlacementConditionType,
		ZoneId:             zoneId,
		Status:             "Done",
		LastTransitionTime: metav1.Now(),
	}
}

func NewOwnershipTransferCondition(zoneId string) v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.OwnershipTransferConditionType,
		ZoneId:             zoneId,
		Status:             string(v1.OwnershipTransferPulling),
		LastTransitionTime: metav1.Now(),
	}
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
		return !(existing.Type == toRemove.Type && existing.ZoneId == toRemove.ZoneId)
	})
}
