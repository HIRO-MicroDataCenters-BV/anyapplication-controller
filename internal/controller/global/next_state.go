package global

// import (
// 	"github.com/samber/mo"
// 	v1 "hiro.io/anyapplication/api/v1"
// 	"hiro.io/anyapplication/internal/clock"
// 	"hiro.io/anyapplication/internal/config"
// 	"hiro.io/anyapplication/internal/controller/job"
// )

// type NextJobs2 struct {
// 	jobsToAdd    mo.Option[job.AsyncJob]
// 	jobsToRemove mo.Option[job.AsyncJobType]
// }

// type nextStateResult struct {
// 	nextState          mo.Option[v1.GlobalState]
// 	conditionsToAdd    mo.Option[*v1.ConditionStatus]
// 	conditionsToRemove mo.Option[*v1.ConditionStatus]
// 	jobs               NextJobs2
// }

// // TODO Consider automatic condition cleanup from jobs

// func nextState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	status := &application.Status

// 	switch status.State {
// 	case v1.NewGlobalState:
// 		// if current state is new and current node is owner
// 		status.State = v1.PlacementGlobalState
// 		return handlePlacementState(application, config, jobFactory, clock, applicationPresent)

// 	case v1.PlacementGlobalState:
// 		return handlePlacementState(application, config, jobFactory, clock, applicationPresent)

// 	case v1.OperationalGlobalState:
// 		return handleOperationalState(application, config, jobFactory, clock, applicationPresent)

// 	case v1.FailureGlobalState:
// 		return handleFailureState(application, config, jobFactory, clock, applicationPresent)

// 	case v1.RelocationGlobalState:
// 		return handleRelocationState(application, config, jobFactory, clock, applicationPresent)

// 	case v1.OwnershipTransferGlobalState:
// 		return handleOwnershipTransferState(application, config, jobFactory, clock, applicationPresent)

// 	default:
// 	}
// 	return nextStateResult{}
// }

// func handlePlacementState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	status := &application.Status
// 	spec := &application.Spec

// 	// if current state is Placement and current node is owner
// 	if spec.PlacementStrategy.Strategy == v1.PlacementStrategyLocal {
// 		// Local Placement strategy
// 		if !conditionExists(status, v1.PlacementConditionType, config.ZoneId) {
// 			placementJob := jobFactory.CreateLocalPlacementJob(application)
// 			condition := placementJob.GetStatus()
// 			return nextStateResult{
// 				nextState:       mo.Some(v1.PlacementGlobalState),
// 				conditionsToAdd: mo.Some(&condition),
// 				jobs: NextJobs2{
// 					jobsToAdd: mo.Some(placementJob),
// 				},
// 			}
// 		}
// 	}
// 	if len(status.Placements) == 0 {
// 		// Wait for global placement strategy to decide about placement
// 		return nextStateResult{
// 			nextState: mo.Some(v1.PlacementGlobalState),
// 		}
// 	}
// 	if placementsContainZone(status, config.ZoneId) {
// 		return handleOperationalState(application, config, jobFactory, clock, applicationPresent)
// 	}
// 	return nextStateResult{
// 		nextState: mo.Some(v1.PlacementGlobalState),
// 	}
// }

// func handleOperationalState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	status := &application.Status

// 	if placementsContainZone(status, config.ZoneId) {
// 		if !applicationPresent {
// 			return handleRelocationState(application, config, jobFactory, clock, applicationPresent)
// 		}

// 		// TODO if local application present
// 		_, foundLocal := getCondition(status, v1.LocalConditionType, config.ZoneId)
// 		if !foundLocal {
// 			operationJob := jobFactory.CreateOperationJob(application)
// 			operationCondition := operationJob.GetStatus()
// 			return nextStateResult{
// 				nextState:       mo.Some(v1.OperationalGlobalState),
// 				conditionsToAdd: mo.Some(&operationCondition),
// 				jobs: NextJobs2{
// 					jobsToAdd: mo.Some(operationJob),
// 				},
// 			}
// 		}

// 		if isFailureCondition(application) {
// 			return handleFailureState(application, config, jobFactory, clock, applicationPresent)
// 		}
// 	}
// 	return nextStateResult{
// 		nextState: mo.Some(v1.OperationalGlobalState),
// 	}
// }

// func handleRelocationState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	status := &application.Status

// 	condition, found := getCondition(status, v1.RelocationConditionType, config.ZoneId)
// 	if !found {
// 		relocationJob := jobFactory.CreateRelocationJob(application)
// 		condition := relocationJob.GetStatus()
// 		return nextStateResult{
// 			nextState:       mo.Some(v1.RelocationGlobalState),
// 			conditionsToAdd: mo.Some(&condition),
// 			jobs: NextJobs2{
// 				jobsToAdd: mo.Some(relocationJob),
// 			},
// 		}
// 	}
// 	switch condition.Status {
// 	case string(v1.RelocationStatusDone):
// 		if applicationPresent {
// 			return handleOperationalState(application, config, jobFactory, clock, applicationPresent)
// 		}

// 	case string(v1.RelocationStatusFailure):
// 		relocationJob := jobFactory.CreateRelocationJob(application) // Add attempt counter
// 		condition := relocationJob.GetStatus()
// 		return nextStateResult{
// 			nextState:       mo.Some(v1.RelocationGlobalState),
// 			conditionsToAdd: mo.Some(&condition),
// 			jobs: NextJobs2{
// 				jobsToAdd: mo.Some(relocationJob),
// 			},
// 		}
// 	}
// 	return nextStateResult{
// 		nextState: mo.Some(v1.RelocationGlobalState),
// 	}
// }

// func handleFailureState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	if !isFailureCondition(application) {
// 		return handleOperationalState(application, config, jobFactory, clock, applicationPresent)
// 	}
// 	return nextStateResult{
// 		nextState: mo.Some(v1.FailureGlobalState),
// 	}
// }

// func handleOwnershipTransferState(
// 	application *v1.AnyApplication,
// 	config *config.ApplicationRuntimeConfig,
// 	jobFactory job.AsyncJobFactory,
// 	clock clock.Clock,
// 	applicationPresent bool,
// ) nextStateResult {
// 	status := &application.Status

// 	if placementsContainZone(status, config.ZoneId) {
// 		condition, found := getCondition(status, v1.LocalConditionType, config.ZoneId)
// 		if found {
// 			if condition.Status == string(v1.OwnershipTransferSuccess) {
// 				return handleOperationalState(application, config, jobFactory, clock, applicationPresent)
// 			}
// 			// else {
// 			// 	// TODO failure case
// 			// }
// 		}
// 		return nextStateResult{
// 			nextState: mo.Some(v1.OperationalGlobalState),
// 			jobs: NextJobs2{
// 				jobsToRemove: mo.Some(job.AsyncJobTypeOwnershipTransfer),
// 			},
// 		}
// 	} else {
// 		if !applicationPresent {

// 		}
// 		/* condition */ _, found := getCondition(status, v1.LocalConditionType, config.ZoneId)

// 		// if condition.Status == string(v1.OwnershipTransferFailure) {
// 		// 	// TODO failure case
// 		// }
// 		var conditionToAdd mo.Option[*v1.ConditionStatus]
// 		transferJob := mo.None[job.AsyncJob]()
// 		if !found {
// 			job := jobFactory.CreateRelocationJob(application)
// 			jobStatus := job.GetStatus()
// 			conditionToAdd = mo.Some(&jobStatus)
// 			transferJob = mo.Some(job)
// 		}
// 		return nextStateResult{
// 			nextState:       mo.Some(v1.OwnershipTransferGlobalState),
// 			conditionsToAdd: conditionToAdd,
// 			jobs: NextJobs2{
// 				jobsToAdd: transferJob,
// 			},
// 		}
// 	}
// }

// func updateJobConditions(status *v1.AnyApplicationStatus, jobConditions JobApplicationConditions) bool {
// 	stateUpdated := false
// 	for _, condition := range jobConditions.Conditions {
// 		addOrUpdateCondition(status, condition)
// 		stateUpdated = true
// 	}
// 	return stateUpdated
// }
