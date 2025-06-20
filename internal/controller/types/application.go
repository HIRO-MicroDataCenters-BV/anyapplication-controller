package types

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
)

type NextJobs struct {
	JobsToAdd    mo.Option[AsyncJob]
	JobsToRemove mo.Option[AsyncJobType]
}

func (n *NextJobs) Add(other NextJobs) {
	if n.JobsToAdd.IsAbsent() {
		n.JobsToAdd = other.JobsToAdd
	} else if other.JobsToAdd.IsPresent() {
		panic("Multiple jobs to schedule")
	}

	if n.JobsToRemove.IsAbsent() {
		n.JobsToRemove = other.JobsToRemove
	} else if other.JobsToRemove.IsPresent() {
		panic("Multiple jobs to unschedule")
	}
}

type NextStateResult struct {
	NextState          mo.Option[v1.GlobalState]
	ConditionsToAdd    mo.Option[*v1.ConditionStatus]
	ConditionsToRemove []*v1.ConditionStatus
	Jobs               NextJobs
}

type StatusResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
	Jobs   NextJobs
}

type GlobalApplication interface {
	GetName() string
	GetNamespace() string
	IsDeployed() bool
	DeriveNewStatus(jobConditions JobApplicationCondition, jobFactory AsyncJobFactory) StatusResult
}
