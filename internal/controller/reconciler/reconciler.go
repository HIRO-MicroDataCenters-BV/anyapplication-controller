package reconciler

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/moutils"
)

type ReconcilerResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
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

func (r *Reconciler) DoReconcile(globalApplication *global.GlobalApplication) ReconcilerResult {
	applicationId := job.ApplicationId{
		Name:      globalApplication.GetName(),
		Namespace: globalApplication.GetNamespace(),
	}
	jobConditions := moutils.
		Map(r.jobs.GetCurrent(applicationId), func(j job.AsyncJob) global.JobApplicationConditions {
			condition := j.GetStatus()
			return global.FromCondition(&condition)
		}).
		OrElse(global.EmptyJobConditions())

	statusResult := globalApplication.DeriveNewStatus(jobConditions, r.jobFactory)
	statusResult.Jobs.JobsToAdd.ForEach(func(newJob job.AsyncJob) {
		r.jobs.Execute(newJob)
	})

	reconcilerResult := ReconcilerResult{
		Status: statusResult.Status,
	}
	return reconcilerResult
}
