package global

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/moutils"
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
	clock           clock.Clock
}

func LoadCurrentState(
	ctx context.Context,
	clock clock.Clock,
	client client.Client,
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
) (GlobalApplication, error) {
	localApplication, err := local.LoadCurrentState(ctx, client, &application.Spec.Application, config)
	if err != nil {
		return GlobalApplication{}, errors.Wrap(err, "Failed to load local application")
	}
	return NewFromLocalApplication(localApplication, clock, application, config), nil
}

func NewFromLocalApplication(
	localApplication mo.Option[local.LocalApplication],
	clock clock.Clock,
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
) GlobalApplication {
	return GlobalApplication{
		locaApplication: localApplication,
		application:     application,
		config:          config,
		clock:           clock,
	}
}

type StatusResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
	Jobs   NextJobs
}

func (g *GlobalApplication) DeriveNewStatus(
	jobConditions JobApplicationConditions,
	jobFactory job.AsyncJobFactory,
) StatusResult {

	current := &g.application.Status

	if current.State == "" {
		current.Owner = g.config.ZoneId
		current.State = v1.NewGlobalState
	}

	// Update local application status if exists
	localConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
		return l.GetCondition()
	})
	stateUpdated := moutils.Map(localConditionOpt, func(condition v1.ConditionStatus) bool {
		updateLocalCondition(current, &condition, g.config)
		return true
	}).OrElse(false)

	// Update loca job conditions
	stateUpdated = updateJobConditions(current, jobConditions) || stateUpdated
	// fmt.Printf("status updated from job conditions %v\n", stateUpdated)
	// Update global state
	globalStateUpdated, nextJobs := updateGlobalState(g.application, g.config, jobFactory, g.clock, g.locaApplication.IsPresent())
	// fmt.Printf("global state updated %v\n", globalStateUpdated)
	stateUpdated = globalStateUpdated || stateUpdated

	// fmt.Printf("result status %v\n", current)
	status := mo.None[v1.AnyApplicationStatus]()
	if stateUpdated {
		status = mo.Some(*current)
	}
	return StatusResult{
		status, nextJobs,
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
	clock clock.Clock,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	if status.Owner == config.ZoneId {
		return globalStateMachine(application, config, jobFactory, clock, applicationPresent)
	} else if applicationPresent {
		return localStateMachine(application, config, jobFactory, clock, applicationPresent)
	} else {
		// Not owner cannot update the state
		return false, NextJobs{}
	}
}

func globalStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	clock clock.Clock,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	stateUpdated := false
	nextStateResult := nextState(application, config, jobFactory, clock, applicationPresent)
	maybeNextState, conditionsToAdd, conditionsToRemove := nextStateResult.nextState, nextStateResult.conditionsToAdd, nextStateResult.conditionsToRemove
	jobs := nextStateResult.jobs

	fmt.Printf("nextStateResult %v \n", nextStateResult)

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

	if jobs.jobsToAdd.IsPresent() || jobs.jobsToRemove.IsPresent() {
		stateUpdated = true
	}

	return stateUpdated, jobs
}

func localStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	clock clock.Clock,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	stateUpdated := false
	nextStateResult := nextState(application, config, jobFactory, clock, applicationPresent)
	maybeNextState, conditionsToAdd, conditionsToRemove := nextStateResult.nextState, nextStateResult.conditionsToAdd, nextStateResult.conditionsToRemove
	jobs := nextStateResult.jobs

	fmt.Printf("nextStateResult %v \n", nextStateResult)

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

	if jobs.jobsToAdd.IsPresent() || jobs.jobsToRemove.IsPresent() {
		stateUpdated = true
	}

	return stateUpdated, jobs
}
