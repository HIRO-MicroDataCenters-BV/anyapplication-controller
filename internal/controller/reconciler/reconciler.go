package reconciler

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/job"
)

type ReconcilerResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
	Action *job.AsyncJob
}

type Reconciler struct {
	globalApplication global.GlobalApplication
	currentJob        *job.AsyncJob
}

func NewReconciler(globalApplication global.GlobalApplication, currentJob *job.AsyncJob) Reconciler {
	return Reconciler{
		globalApplication: globalApplication,
		currentJob:        currentJob,
	}
}

func (r *Reconciler) DoReconcile() ReconcilerResult {
	return ReconcilerResult{
		Status: mo.Some(v1.AnyApplicationStatus{
			Conditions: []v1.ConditionStatus{},
		}),
	}
}
