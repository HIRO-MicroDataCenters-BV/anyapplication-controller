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
	status := g.application.Status

	if !placementsContainZone(&status, g.config.ZoneId) && g.applicationPresent {
		return g.handleUndeploy()
	}

	if placementsContainZone(&status, g.config.ZoneId) {
		return g.handleOperation()
	}

	return types.NextStateResult{}
}

func (g *LocalFSM) handleUndeploy() types.NextStateResult {
	status := g.application.Status

	operationCondition, _ := getCondition(status.Conditions, v1.LocalConditionType, g.config.ZoneId)

	undeployCondition, found := getCondition(status.Conditions, v1.RelocationConditionType, g.config.ZoneId)
	undeployConditionOpt := mo.EmptyableToOption(undeployCondition)
	undeployJobOpt := mo.None[types.AsyncJob]()

	if !found || !g.isRunning(types.AsyncJobTypeRelocate) {
		undeployJob := g.jobFactory.CreateUndeployJob(g.application)
		undeployCondition := undeployJob.GetStatus()

		undeployConditionOpt = mo.Some(&undeployCondition)
		undeployJobOpt = mo.Some(undeployJob)
	}

	return types.NextStateResult{
		ConditionsToAdd:    undeployConditionOpt,
		ConditionsToRemove: mo.EmptyableToOption(operationCondition),
		Jobs:               types.NextJobs{JobsToAdd: undeployJobOpt},
	}
}

func (g *LocalFSM) handleOperation() types.NextStateResult {
	status := g.application.Status

	_, found := getCondition(status.Conditions, v1.LocalConditionType, g.config.ZoneId)
	if !found || !g.isRunning(types.AsyncJobTypeLocalOperation) {

		if !g.applicationPresent {
			relocationJob := g.jobFactory.CreateRelocationJob(g.application)
			relocationCondition := relocationJob.GetStatus()
			relocationConditionOpt := mo.Some(&relocationCondition)
			return types.NextStateResult{
				ConditionsToAdd: relocationConditionOpt,
				Jobs:            types.NextJobs{JobsToAdd: mo.Some(relocationJob)},
			}

		} else {
			operationJob := g.jobFactory.CreateOperationJob(g.application)
			operationCondition := operationJob.GetStatus()

			operationConditionOpt := mo.Some(&operationCondition)
			return types.NextStateResult{
				ConditionsToAdd: operationConditionOpt,
				Jobs:            types.NextJobs{JobsToAdd: mo.Some(operationJob)},
			}

		}
	} else {
		return types.NextStateResult{}
	}
}

func (g *LocalFSM) isRunning(jobType types.AsyncJobType) bool {
	return g.runningJobType.OrEmpty() == jobType
}
