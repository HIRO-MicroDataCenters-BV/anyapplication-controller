/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/sync"
)

// AnyApplicationReconciler reconciles a AnyApplication object
type AnyApplicationReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Config      *config.ApplicationRuntimeConfig
	SyncManager sync.SyncManager
	Jobs        job.AsyncJobs
	Reconciler  reconciler.Reconciler
}

// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AnyApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *AnyApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	resource := &dcpv1.AnyApplication{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		log.Error(err, "Unable to get AnyApplication ", "name", req.Name, "namespace", req.Namespace)
		// TODO (user): handle error
		return ctrl.Result{}, nil
	}

	// globalApplication, err := r.SyncManager.LoadApplication(resource)
	// if err != nil {
	// 	log.Error(err, "failed to update AnyApplication status")
	// 	return ctrl.Result{}, err
	// }

	// result := r.Reconciler.DoReconcile(globalApplication)

	// resource.Status = result.Status.OrElse(resource.Status)

	// err = r.Client.Status().Update(ctx, resource)
	// if err != nil {
	// 	log.Error(err, "failed to update AnyApplication status")
	// 	return ctrl.Result{}, err
	// }

	log.Info("AnyApplicaitonResource status synced")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnyApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dcpv1.AnyApplication{}).
		Named("anyapplication").
		Complete(r)
}
