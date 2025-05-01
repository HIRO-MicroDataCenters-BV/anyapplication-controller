package reconciler

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/job"
)

type ReconcilerResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
	Job    job.AsyncJob
}

type Reconciler struct {
	jobFactory job.AsyncJobFactory
	jobs       job.AsyncJobs
}

func NewReconciler(jobs job.AsyncJobs, jobFactory job.AsyncJobFactory) Reconciler {
	return Reconciler{
		jobs:       jobs,
		jobFactory: jobFactory,
	}
}

func (r *Reconciler) DoReconcile(globalApplication global.GlobalApplication, currentJob job.AsyncJob) global.StatusResult {
	jobConditions := global.EmptyJobConditions()
	statusResult := globalApplication.DeriveNewStatus(jobConditions, r.jobFactory)
	return statusResult
}
