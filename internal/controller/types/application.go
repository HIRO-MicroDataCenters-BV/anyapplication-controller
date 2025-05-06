package types

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
)

type NextJobs struct {
	JobsToAdd    mo.Option[AsyncJob]
	JobsToRemove mo.Option[AsyncJobType]
}

type NextStateResult struct {
	NextState          mo.Option[v1.GlobalState]
	ConditionsToAdd    mo.Option[*v1.ConditionStatus]
	ConditionsToRemove mo.Option[*v1.ConditionStatus]
	Jobs               NextJobs
}

type StatusResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
	Jobs   NextJobs
}

type GlobalApplication interface {
	GetName() string
	GetNamespace() string
	DeriveNewStatus(jobConditions JobApplicationCondition, jobFactory AsyncJobFactory) StatusResult
}
