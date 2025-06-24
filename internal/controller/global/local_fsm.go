package global

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	types "hiro.io/anyapplication/internal/controller/types"
)

type LocalFSM struct {
	application        *v1.AnyApplication
	config             *config.ApplicationRuntimeConfig
	jobFactory         types.AsyncJobFactory
	applicationPresent bool
	runningJobType     mo.Option[types.AsyncJobType]
}

func NewLocalFSM(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory types.AsyncJobFactory,
	applicationPresent bool,
	runningJobType mo.Option[types.AsyncJobType],
) LocalFSM {
	return LocalFSM{
		application, config, jobFactory, applicationPresent, runningJobType,
	}
}

func (g *LocalFSM) NextState() types.NextStateResult {
	status := &g.application.Status

	if !placementsContainZone(status, g.config.ZoneId) && g.applicationPresent {
		return g.handleUndeploy()
	}

	zoneStatus := status.GetOrCreateStatusFor(g.config.ZoneId)

	isDeploymentSucceeded := deploymentSuccessfull(zoneStatus, g.config.ZoneId)
	if placementsContainZone(status, g.config.ZoneId) && !isDeploymentSucceeded {
		return g.handleDeploy()
	}

	if placementsContainZone(status, g.config.ZoneId) {
		return g.handleOperation()
	}
	return types.NextStateResult{}
}

func (g *LocalFSM) handleDeploy() types.NextStateResult {
	status := g.application.Status.GetOrCreateStatusFor(g.config.ZoneId)

	conditionsToRemove := make([]*v1.ConditionStatus, 0)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.LocalConditionType, g.config.ZoneId)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.UndeploymenConditionType, g.config.ZoneId)

	isDeploymentSucceeded := deploymentSuccessfull(status, g.config.ZoneId)
	if !g.applicationPresent && !isDeploymentSucceeded {
		if !g.isRunning(types.AsyncJobTypeDeploy) {
			deployJob := g.jobFactory.CreateDeploymentJob(g.application)
			deployJobOpt := mo.Some(deployJob)
			deployCondition := deployJob.GetStatus()
			deployConditionOpt := mo.EmptyableToOption(&deployCondition)

			return types.NextStateResult{
				ConditionsToAdd:    deployConditionOpt,
				ConditionsToRemove: conditionsToRemove,
				Jobs:               types.NextJobs{JobsToAdd: deployJobOpt},
			}
		}
	} else if g.applicationPresent {
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
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.DeploymenConditionType, g.config.ZoneId)

	if !g.isRunning(types.AsyncJobTypeUndeploy) {

		if g.applicationPresent {
			undeployJob := g.jobFactory.CreateUndeployJob(g.application)
			undeployJobOpt := mo.Some(undeployJob)
			undeployCondition := undeployJob.GetStatus()
			undeployConditionOpt := mo.EmptyableToOption(&undeployCondition)

			return types.NextStateResult{
				ConditionsToAdd:    undeployConditionOpt,
				ConditionsToRemove: conditionsToRemove,
				Jobs:               types.NextJobs{JobsToAdd: undeployJobOpt},
			}
		}
	}

	return types.NextStateResult{
		ConditionsToRemove: conditionsToRemove,
	}
}

func (g *LocalFSM) handleOperation() types.NextStateResult {
	status := g.application.Status.GetOrCreateStatusFor(g.config.ZoneId)

	conditionsToRemove := make([]*v1.ConditionStatus, 0)
	conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.UndeploymenConditionType, g.config.ZoneId)

	isDeploymentSucceeded := deploymentSuccessfull(status, g.config.ZoneId)

	if isDeploymentSucceeded && !g.isRunning(types.AsyncJobTypeLocalOperation) {
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
		conditionsToRemove = addConditionToRemoveList(conditionsToRemove, status.Conditions, v1.DeploymenConditionType, g.config.ZoneId)
	}

	return types.NextStateResult{
		ConditionsToRemove: conditionsToRemove,
	}
}

func (g *LocalFSM) isRunning(jobType types.AsyncJobType) bool {
	return g.runningJobType.OrEmpty() == jobType
}

func deploymentSuccessfull(zoneStatus *v1.ZoneStatus, zoneId string) bool {
	conditions := zoneStatus.Conditions
	deploymentCondition, deploymentConditionFound := getCondition(conditions, v1.DeploymenConditionType, zoneId)
	return deploymentConditionFound && deploymentCondition.Status == string(v1.DeploymentStatusDone)
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
