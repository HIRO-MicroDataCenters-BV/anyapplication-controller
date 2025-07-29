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
)

type globalApplication struct {
	localApplications map[types.SpecificVersion]*local.LocalApplication
	activeVersion     mo.Option[*types.SpecificVersion]
	newVersion        mo.Option[*types.SpecificVersion]
	application       *v1.AnyApplication
	config            *config.ApplicationRuntimeConfig
	clock             clock.Clock
	log               logr.Logger
}

func NewFromLocalApplication(
	localApplications map[types.SpecificVersion]*local.LocalApplication,
	activeVersion mo.Option[*types.SpecificVersion],
	newVersion mo.Option[*types.SpecificVersion],
	clock clock.Clock,
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	log logr.Logger,
) types.GlobalApplication {
	log = log.WithName("GlobalApplication")
	return &globalApplication{
		localApplications: localApplications,
		application:       application,
		activeVersion:     activeVersion,
		newVersion:        newVersion,
		config:            config,
		clock:             clock,
		log:               log,
	}
}

func (g *globalApplication) GetName() string {
	return g.application.Name
}

func (g *globalApplication) GetNamespace() string {
	return g.application.Namespace
}

func (g *globalApplication) IsDeployed() bool {
	if version, present := g.activeVersion.Get(); present {
		if localApplication, present := g.localApplications[*version]; present {
			return localApplication.IsDeployed()
		}
	}
	return false
}

func (g *globalApplication) IsPresent() bool {
	return len(g.localApplications) > 0
}

func (g *globalApplication) IsVersionChanged() bool {
	return g.newVersion.IsPresent()
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

	stateUpdated := false

	// Update local application status if exists
	if localApp, exists := g.getActiveApplication().Get(); exists {
		if localApp.IsDeployed() {
			localAppCondition := localApp.GetCondition()
			updateLocalCondition(current, &localAppCondition, g.config)
			stateUpdated = true
		}
	}

	// Update local job conditions
	stateUpdated = updateJobConditions(current, jobConditions, g.config.ZoneId) || stateUpdated

	// Update state
	globalStateUpdated, nextJobs := updateState(
		g.application,
		g.config,
		jobFactory,
		g.IsPresent(),
		g.IsDeployed(),
		g.IsVersionChanged(),
		g.deriveTargetVersion(),
		g.newVersion,
		runningJobType,
	)

	stateUpdated = globalStateUpdated || stateUpdated

	status := mo.None[v1.AnyApplicationStatus]()
	if stateUpdated {
		status = mo.Some(*current)
	}
	return types.StatusResult{
		Status: status,
		Jobs:   nextJobs,
	}
}

func (g *globalApplication) getActiveApplication() mo.Option[*local.LocalApplication] {
	if version, found := g.activeVersion.Get(); found {
		if localApp, exists := g.localApplications[*version]; exists {
			return mo.Some(localApp)
		}
	}
	return mo.None[*local.LocalApplication]()
}

func (g *globalApplication) deriveTargetVersion() *types.SpecificVersion {
	if version, found := g.newVersion.Get(); found {
		return version
	}
	if version, found := g.activeVersion.Get(); found {
		return version
	}
	panic("No active or new version found for global application: " + g.application.Name)
}

func updateState(
	applicationMut *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	applicationDeployed bool,
	newVersionAvailable bool,
	version *types.SpecificVersion,
	newVersion mo.Option[*types.SpecificVersion],
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {
	status := &applicationMut.Status
	nextJobs := types.NextJobs{}
	stateUpdated := false

	if placementsContainZone(status, config.ZoneId) || applicationPresent || status.ZoneExists(config.ZoneId) {
		stateUpdated, nextJobs = localStateMachine(
			applicationMut,
			config,
			jobFactory,
			applicationPresent,
			applicationDeployed,
			newVersionAvailable,
			version,
			newVersion,
			runningJobType,
		)
	}

	if status.Owner == config.ZoneId {
		globalStateUpdated, globalJobs := globalStateMachine(
			applicationMut,
			config,
			jobFactory,
			applicationDeployed,
			runningJobType,
		)
		stateUpdated = stateUpdated || globalStateUpdated
		nextJobs.Add(globalJobs)
	}

	zoneStatus, exists := applicationMut.Status.GetStatusFor(config.ZoneId)
	if exists {
		if zoneStatus.EmptyConditions() {
			applicationMut.Status.RemoveZone(config.ZoneId)
			stateUpdated = true
		}
	}

	return stateUpdated, nextJobs
}

func globalStateMachine(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationResourcesAvailable bool,
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {
	status := &application.Status

	stateUpdated := false

	fsm := NewGlobalFSM(application, config, jobFactory, applicationResourcesAvailable, runningJobType)
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
	applicationResourcesPresent bool,
	applicationDeployed bool,
	newVersionAvailable bool,
	version *types.SpecificVersion,
	newVersion mo.Option[*types.SpecificVersion],
	runningJobType mo.Option[types.AsyncJobType],
) (bool, types.NextJobs) {

	stateUpdated := false

	fsm := NewLocalFSM(
		applicationMut,
		config,
		jobFactory,
		applicationResourcesPresent,
		applicationDeployed,
		newVersionAvailable,
		version,
		newVersion,
		runningJobType,
	)
	nextStateResult := fsm.NextState()

	conditionsToAdd, conditionsToRemove := nextStateResult.ConditionsToAdd, nextStateResult.ConditionsToRemove
	jobs := nextStateResult.Jobs

	for _, condition := range conditionsToRemove {
		removeCondition(&applicationMut.Status, condition, config.ZoneId)
		stateUpdated = true
	}

	conditionsToAdd.ForEach(func(condition *v1.ConditionStatus) {
		addOrUpdateCondition(&applicationMut.Status, condition, config.ZoneId)
		stateUpdated = true
	})

	version, present := nextStateResult.NewVersion.Get()
	if present {
		setNewVersion(&applicationMut.Status, version, config.ZoneId)
		stateUpdated = true
	}

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
