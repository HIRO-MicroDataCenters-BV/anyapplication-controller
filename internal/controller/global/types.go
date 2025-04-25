package global

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/job"
)

type NextJobs struct {
	JobsToAdd    mo.Option[job.AsyncJob]
	JobsToRemove mo.Option[job.AsyncJobType]
}

type NextStateResult struct {
	NextState          mo.Option[v1.GlobalState]
	ConditionsToAdd    mo.Option[*v1.ConditionStatus]
	ConditionsToRemove mo.Option[*v1.ConditionStatus]
	Jobs               NextJobs
}
