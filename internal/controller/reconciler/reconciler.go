// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/moutils"
)

type ReconcilerResult struct {
	Status       mo.Option[v1.AnyApplicationStatus]
	JobsToAdd    mo.Option[types.AsyncJob]
	JobsToRemove bool
}

type Reconciler struct {
	jobFactory types.AsyncJobFactory
	jobs       types.AsyncJobs
}

func NewReconciler(jobs types.AsyncJobs, jobFactory types.AsyncJobFactory) Reconciler {
	return Reconciler{
		jobs:       jobs,
		jobFactory: jobFactory,
	}
}

func (r *Reconciler) DoReconcile(globalApplication types.GlobalApplication) ReconcilerResult {
	applicationId := types.ApplicationId{
		Name:      globalApplication.GetName(),
		Namespace: globalApplication.GetNamespace(),
	}
	jobConditions := moutils.
		Map(r.jobs.GetCurrent(applicationId), func(j types.AsyncJob) types.JobApplicationCondition {
			condition := j.GetStatus()
			return types.FromCondition(condition, j.GetType())
		}).
		OrElse(types.EmptyJobConditions())

	statusResult := globalApplication.DeriveNewStatus(jobConditions, r.jobFactory)

	reconcilerResult := ReconcilerResult{
		Status:       statusResult.Status,
		JobsToAdd:    statusResult.Jobs.JobsToAdd,
		JobsToRemove: statusResult.Jobs.JobsToRemove.IsPresent(),
	}
	return reconcilerResult
}
