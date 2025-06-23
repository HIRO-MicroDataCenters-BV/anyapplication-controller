package global

import (
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/moutils"
)

type globalApplication struct {
	locaApplication mo.Option[local.LocalApplication]
	application     *v1.AnyApplication
	config          *config.ApplicationRuntimeConfig
	clock           clock.Clock
	log             logr.Logger
}

func NewFromLocalApplication(
	localApplication mo.Option[local.LocalApplication],
	clock clock.Clock,
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	log logr.Logger,

) types.GlobalApplication {
	log = log.WithName("GlobalApplication")
	return &globalApplication{
		locaApplication: localApplication,
		application:     application,
		config:          config,
		clock:           clock,
		log:             log,
	}
}

func (g *globalApplication) GetName() string {
	return g.application.Name
}

func (g *globalApplication) GetNamespace() string {
	return g.application.Namespace
}

func (g *globalApplication) IsDeployed() bool {
	return g.locaApplication.IsPresent()
}

func (g *globalApplication) HasZoneStatus() bool {
	return g.application.HasZoneStatus(g.config.ZoneId)
}

func (g *globalApplication) DeriveNewStatus(
	jobConditions types.JobApplicationCondition,
	jobFactory types.AsyncJobFactory,
) types.StatusResult {

	current := &g.application.Status

	if current.State == "" {
		current.Owner = g.config.ZoneId
		current.State = v1.NewGlobalState
	}
	runningJobType := jobConditions.GetJobType()

	// Update local application status if exists
	localApplicationConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
		return l.GetCondition()
	})
	stateUpdated := moutils.Map(localApplicationConditionOpt, func(condition v1.ConditionStatus) bool {
		updateLocalCondition(current, &condition, g.config)
		return true
	}).OrElse(false)

	// Update loca job conditions
	stateUpdated = updateJobConditions(current, jobConditions, g.config.ZoneId) || stateUpdated

	// Update state
	globalStateUpdated, nextJobs := updateState(g.application, g.config, jobFactory, g.IsDeployed(), runningJobType)

	stateUpdated = globalStateUpdated || stateUpdated

	status := mo.None[v1.AnyApplicationStatus]()
	if stateUpdated {
		status = mo.Some(*current)
	}
	return types.StatusResult{
		Status: status, Jobs: nextJobs,
	}
}

func updateState(
	applicationMut *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {
	status := &applicationMut.Status
	nextJobs := types.NextJobs{}
	stateUpdated := false
	if placementsContainZone(status, config.ZoneId) || applicationPresent {
		stateUpdated, nextJobs = localStateMachine(
			applicationMut,
			config,
			jobFactory,
			applicationPresent,
			runningJobType,
		)
	}

	if status.Owner == config.ZoneId {
		globalStateUpdated, globalJobs := globalStateMachine(
			applicationMut,
			config,
			jobFactory,
			applicationPresent,
			runningJobType,
		)
		stateUpdated = stateUpdated || globalStateUpdated
		nextJobs.Add(globalJobs)
	}

	return stateUpdated, nextJobs
}

func globalStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {
	status := &application.Status

	stateUpdated := false

	fsm := NewGlobalFSM(application, config, jobFactory, applicationPresent, runningJobType)
	nextStateResult := fsm.NextState()

	maybeNextState, conditionsToAdd, conditionsToRemove := nextStateResult.NextState, nextStateResult.ConditionsToAdd, nextStateResult.ConditionsToRemove

	jobs := nextStateResult.Jobs
	if jobs.JobsToAdd.IsPresent() || jobs.JobsToRemove.IsPresent() {
		stateUpdated = true
	}

	// TODO pick condition from jobs
	for _, condition := range conditionsToRemove {
		removeCondition(status, condition, config.ZoneId)
		stateUpdated = true
	}

	// TODO pick condition from jobs
	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(status, condition, config.ZoneId)
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
	applicationMut *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {

	stateUpdated := false

	fsm := NewLocalFSM(applicationMut, config, jobFactory, applicationPresent, runningJobType)
	nextStateResult := fsm.NextState()

	conditionsToAdd, conditionsToRemove := nextStateResult.ConditionsToAdd, nextStateResult.ConditionsToRemove
	jobs := nextStateResult.Jobs

	// TODO pick condition from jobs
	for _, condition := range conditionsToRemove {
		removeCondition(&applicationMut.Status, condition, config.ZoneId)
		stateUpdated = true
	}

	// TODO pick condition from jobs
	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(&applicationMut.Status, condition, config.ZoneId)
		stateUpdated = true
	})

	if jobs.JobsToAdd.IsPresent() || jobs.JobsToRemove.IsPresent() {
		stateUpdated = true
	}

	return stateUpdated, jobs
}

func updateJobConditions(status *v1.AnyApplicationStatus, jobConditions types.JobApplicationCondition, zoneId string) bool {
	stateUpdated := false

	for _, condition := range jobConditions.GetConditions() {
		addOrUpdateCondition(status, condition, zoneId)
		stateUpdated = true
	}
	return stateUpdated
}

func updateLocalCondition(status *v1.AnyApplicationStatus, condition *v1.ConditionStatus, config *config.ApplicationRuntimeConfig) {
	zoneStatus := status.GetOrCreateStatusFor(config.ZoneId)

	found, ok := lo.Find(zoneStatus.Conditions, func(cond v1.ConditionStatus) bool {
		return cond.ZoneId == config.ZoneId && cond.Type == condition.Type
	})
	if !ok {
		zoneStatus.Conditions = append(zoneStatus.Conditions, *condition)
	} else {
		condition.DeepCopyInto(&found)
	}
}
