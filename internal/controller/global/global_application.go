package global

import (
	"context"

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

func (g *GlobalApplication) GetName() string {
	panic("not implemented")
}

func (g *GlobalApplication) GetNamespace() string {
	panic("not implemented")
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

	// Update global state
	globalStateUpdated, nextJobs := updateGlobalState(g.application, g.config, jobFactory, g.locaApplication.IsPresent())

	stateUpdated = globalStateUpdated || stateUpdated

	status := mo.None[v1.AnyApplicationStatus]()
	if stateUpdated {
		status = mo.Some(*current)
	}
	return StatusResult{
		status, nextJobs,
	}
}

func updateGlobalState(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	if status.Owner == config.ZoneId {
		return globalStateMachine(application, config, jobFactory, applicationPresent)
	} else if applicationPresent || placementsContainZone(status, config.ZoneId) {
		return localStateMachine(application, config, jobFactory, applicationPresent)
	} else {
		return false, NextJobs{}
	}
}

func globalStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	stateUpdated := false

	fsm := NewGlobalFSM(application, config, jobFactory, applicationPresent)
	nextStateResult := fsm.NextState()

	maybeNextState, conditionsToAdd, conditionsToRemove := nextStateResult.NextState, nextStateResult.ConditionsToAdd, nextStateResult.ConditionsToRemove

	jobs := nextStateResult.Jobs
	if jobs.JobsToAdd.IsPresent() || jobs.JobsToRemove.IsPresent() {
		stateUpdated = true
	}

	// TODO pick condition from jobs
	conditionsToRemove.ForEach(func(condition *v1.ConditionStatus) {
		removeCondition(status, condition)
		stateUpdated = true
	})

	// TODO pick condition from jobs
	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(status, condition)
		stateUpdated = true
	})

	nextState := maybeNextState.OrElse(status.State)
	if status.State != nextState {
		status.State = nextState
		stateUpdated = true
	}

	return stateUpdated, jobs
}

func localStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	applicationPresent bool,
) (bool, NextJobs) {
	status := &application.Status

	stateUpdated := false

	fsm := NewLocalFSM(application, config, jobFactory, applicationPresent)
	nextStateResult := fsm.NextState()

	conditionsToAdd, conditionsToRemove := nextStateResult.ConditionsToAdd, nextStateResult.ConditionsToRemove
	jobs := nextStateResult.Jobs

	// TODO pick condition from jobs
	conditionsToRemove.ForEach(func(condition *v1.ConditionStatus) {
		removeCondition(status, condition)
		stateUpdated = true
	})

	// TODO pick condition from jobs
	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(status, condition)
		stateUpdated = true
	})

	if jobs.JobsToAdd.IsPresent() || jobs.JobsToRemove.IsPresent() {
		stateUpdated = true
	}

	return stateUpdated, jobs
}

func updateJobConditions(status *v1.AnyApplicationStatus, jobConditions JobApplicationConditions) bool {
	stateUpdated := false
	for _, condition := range jobConditions.Conditions {
		addOrUpdateCondition(status, condition)
		stateUpdated = true
	}
	return stateUpdated
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
