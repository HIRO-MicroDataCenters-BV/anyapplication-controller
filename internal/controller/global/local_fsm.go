package global

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	types "hiro.io/anyapplication/internal/controller/types"
)

type LocalFSM struct {
	application              *v1.AnyApplication
	recoverStrategy          *v1.RecoverStrategySpec
	config                   *config.ApplicationRuntimeConfig
	jobFactory               types.AsyncJobFactory
	applicationPresent       bool
	applicationDeployed      bool
	nonActiveVersionsPresent bool
	newVersionAvailable      bool
	version                  *types.SpecificVersion
	newVersion               mo.Option[*types.SpecificVersion]
	runningJobType           mo.Option[types.AsyncJobType]
}

func NewLocalFSM(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	applicationDeployed bool,
	nonActiveVersionsPresent bool,
	newVersionAvailable bool, // TODO remove this and derive from newVersion
	version *types.SpecificVersion,
	newVersion mo.Option[*types.SpecificVersion],
	runningJobType mo.Option[types.AsyncJobType],
) *LocalFSM {
	recoverStrategy := &application.Spec.RecoverStrategy
	return &LocalFSM{
		application:              application,
		recoverStrategy:          recoverStrategy,
		config:                   config,
		jobFactory:               jobFactory,
		applicationPresent:       applicationPresent,
		applicationDeployed:      applicationDeployed,
		nonActiveVersionsPresent: nonActiveVersionsPresent,
		newVersionAvailable:      newVersionAvailable,
		version:                  version,
		newVersion:               newVersion,
		runningJobType:           runningJobType,
	}
}

func (g *LocalFSM) NextState() types.NextStateResult {
	status := &g.application.Status

	placementsContainZone := placementsContainZone(status, g.config.ZoneId)

	undeployOldVersion := g.applicationPresent && g.newVersionAvailable

	if !placementsContainZone && g.applicationPresent || g.nonActiveVersionsPresent || undeployOldVersion {
		return g.handleUndeploy()
	}

	if placementsContainZone {
		if !g.applicationDeployed {
			return g.handleDeploy()
		} else {
			return g.handleOperation()
		}
	}

	if !placementsContainZone && !g.applicationDeployed && !g.applicationPresent {
		if zoneStatus, exists := status.GetStatusFor(g.config.ZoneId); exists {
			conditionsToRemove := make([]*v1.ConditionStatus, 0)
			for _, condition := range zoneStatus.Conditions {
				conditionsToRemove = append(conditionsToRemove, &condition)
			}
			return types.NextStateResult{ConditionsToRemove: conditionsToRemove}

		}
	}
	return types.NextStateResult{}
}

func (g *LocalFSM) handleDeploy() types.NextStateResult {
	status := g.application.Status.GetOrCreateStatusFor(g.config.ZoneId)

	conditionsToRemove := make([]*v1.ConditionStatus, 0)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.LocalConditionType, g.config.ZoneId)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.UndeploymentConditionType, g.config.ZoneId)

	if !g.applicationDeployed || g.newVersionAvailable {
		if !g.isRunning(types.AsyncJobTypeDeploy) {
			deploymentCondition, found := status.FindCondition(v1.DeploymentConditionType)
			attemptsExhausted := false
			if found {
				attemptsExhausted = deploymentCondition.Status == string(v1.DeploymentStatusFailure) &&
					deploymentCondition.RetryAttempt >= g.recoverStrategy.MaxRetries
			}

			if !attemptsExhausted {
				version := g.version
				newVersion, present := g.newVersion.Get()
				if g.newVersionAvailable && present {
					version = newVersion
				}
				deployJob := g.jobFactory.CreateDeployJob(g.application, version)
				deployJobOpt := mo.Some(deployJob)
				deployCondition := deployJob.GetStatus()
				deployConditionOpt := mo.EmptyableToOption(&deployCondition)

				return types.NextStateResult{
					ConditionsToAdd:    deployConditionOpt,
					ConditionsToRemove: conditionsToRemove,
					Jobs:               types.NextJobs{JobsToAdd: deployJobOpt},
					NewVersion:         g.newVersion,
				}
			}
		}
	} else if g.applicationDeployed {
		return g.handleOperation()
	}

	return types.NextStateResult{
		ConditionsToRemove: conditionsToRemove,
	}
}

func (g *LocalFSM) handleUndeploy() types.NextStateResult {
	status := g.application.Status.GetOrCreateStatusFor(g.config.ZoneId)

	conditionsToRemove := make([]*v1.ConditionStatus, 0)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.LocalConditionType, g.config.ZoneId)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.DeploymentConditionType, g.config.ZoneId)

	undeploymentCondition, found := status.FindCondition(v1.UndeploymentConditionType)
	attemptsExhausted := false

	if found {
		attemptsExhausted = undeploymentCondition.Status == string(v1.UndeploymentStatusFailure) &&
			undeploymentCondition.RetryAttempt >= g.recoverStrategy.MaxRetries
	}

	if !g.isRunning(types.AsyncJobTypeUndeploy) {

		if g.applicationPresent && !attemptsExhausted {
			newVersion := mo.None[*types.SpecificVersion]()
			if g.newVersionAvailable {
				newVersion = mo.Some(g.version)
			}
			undeployJob := g.jobFactory.CreateUndeployJob(g.application)
			undeployJobOpt := mo.Some(undeployJob)
			undeployCondition := undeployJob.GetStatus()
			undeployConditionOpt := mo.EmptyableToOption(&undeployCondition)

			return types.NextStateResult{
				ConditionsToAdd:    undeployConditionOpt,
				ConditionsToRemove: conditionsToRemove,
				Jobs:               types.NextJobs{JobsToAdd: undeployJobOpt},
				NewVersion:         newVersion,
			}
		}
	}

	if found && undeploymentCondition.Status == string(v1.UndeploymentStatusDone) {
		conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.UndeploymentConditionType, g.config.ZoneId)
	}

	return types.NextStateResult{
		ConditionsToRemove: conditionsToRemove,
	}
}

func (g *LocalFSM) handleOperation() types.NextStateResult {
	status := g.application.Status.GetOrCreateStatusFor(g.config.ZoneId)

	conditionsToRemove := make([]*v1.ConditionStatus, 0)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.UndeploymentConditionType, g.config.ZoneId)

	if g.applicationDeployed && !g.isRunning(types.AsyncJobTypeLocalOperation) {
		operationJob := g.jobFactory.CreateOperationJob(g.application)
		operationCondition := operationJob.GetStatus()

		operationConditionOpt := mo.Some(&operationCondition)
		return types.NextStateResult{
			ConditionsToAdd:    operationConditionOpt,
			ConditionsToRemove: conditionsToRemove,
			Jobs:               types.NextJobs{JobsToAdd: mo.Some(operationJob)},
		}
	}

	// By default remove deployment conditions
	if g.isRunning(types.AsyncJobTypeLocalOperation) || len(conditionsToRemove) > 0 {
		conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.DeploymentConditionType, g.config.ZoneId)
	}

	return types.NextStateResult{
		ConditionsToRemove: conditionsToRemove,
	}
}

func (g *LocalFSM) isRunning(jobType types.AsyncJobType) bool {
	return g.runningJobType.OrEmpty() == jobType
}

func addConditionToRemoveList(
	conditionsToRemove []*v1.ConditionStatus,
	conditions []v1.ConditionStatus,
	conditionType v1.ApplicationConditionType,
	zoneId string,
) []*v1.ConditionStatus {
	condition, found := getCondition(conditions, conditionType, zoneId)
	if found {
		conditionsToRemove = append(conditionsToRemove, condition)
	}
	return conditionsToRemove
}
