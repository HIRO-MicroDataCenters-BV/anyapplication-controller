package global

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
)

type LocalFSM struct {
	application        *v1.AnyApplication
	config             *config.ApplicationRuntimeConfig
	jobFactory         job.AsyncJobFactory
	applicationPresent bool
}

func NewLocalFSM(
	application *v1.AnyApplication,
	config *config.ApplicationRuntimeConfig,
	jobFactory job.AsyncJobFactory,
	applicationPresent bool,
) LocalFSM {
	return LocalFSM{
		application, config, jobFactory, applicationPresent,
	}
}

func (g *LocalFSM) NextState() NextStateResult {
	status := g.application.Status

	if !placementsContainZone(&status, g.config.ZoneId) && g.applicationPresent {
		return g.handleUndeploy()
	}

	if placementsContainZone(&status, g.config.ZoneId) {
		return g.handleOperation()
	}

	return NextStateResult{}
}

func (g *LocalFSM) handleUndeploy() NextStateResult {
	status := g.application.Status

	operationCondition, _ := getCondition(&status, v1.LocalConditionType, g.config.ZoneId)

	undeployCondition, found := getCondition(&status, v1.RelocationConditionType, g.config.ZoneId)
	undeployConditionOpt := mo.EmptyableToOption(undeployCondition)
	undeployJobOpt := mo.None[job.AsyncJob]()
	if !found {
		undeployJob := g.jobFactory.CreateUndeployJob(g.application)
		undeployCondition := undeployJob.GetStatus()

		undeployConditionOpt = mo.Some(&undeployCondition)
		undeployJobOpt = mo.Some(undeployJob)
	}

	return NextStateResult{
		ConditionsToAdd:    undeployConditionOpt,
		ConditionsToRemove: mo.EmptyableToOption(operationCondition),
		Jobs:               NextJobs{JobsToAdd: undeployJobOpt},
	}
}

func (g *LocalFSM) handleOperation() NextStateResult {
	status := g.application.Status

	_, found := getCondition(&status, v1.LocalConditionType, g.config.ZoneId)
	if !found {

		if !g.applicationPresent {
			relocationJob := g.jobFactory.CreateRelocationJob(g.application)
			relocationCondition := relocationJob.GetStatus()
			relocationConditionOpt := mo.Some(&relocationCondition)
			return NextStateResult{
				ConditionsToAdd: relocationConditionOpt,
				Jobs:            NextJobs{JobsToAdd: mo.Some(relocationJob)},
			}

		} else {
			operationJob := g.jobFactory.CreateOperationJob(g.application)
			operationCondition := operationJob.GetStatus()

			operationConditionOpt := mo.Some(&operationCondition)
			return NextStateResult{
				ConditionsToAdd: operationConditionOpt,
				Jobs:            NextJobs{JobsToAdd: mo.Some(operationJob)},
			}

		}
	} else {
		return NextStateResult{}
	}
}
