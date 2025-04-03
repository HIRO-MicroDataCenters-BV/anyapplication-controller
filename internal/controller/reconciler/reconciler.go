package reconciler

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/global"
)

type ReconcilerResult struct {
	Status mo.Option[v1.AnyApplicationStatus]
}

type Reconciler struct {
	globalApplication global.GlobalApplication
}

func NewReconciler(globalApplication global.GlobalApplication) Reconciler {
	return Reconciler{
		globalApplication: globalApplication,
	}
}

func (r *Reconciler) DoReconcile() ReconcilerResult {
	return ReconcilerResult{
		Status: mo.Some(v1.AnyApplicationStatus{
			Conditions: []v1.ConditionStatus{},
		}),
	}
}
