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
	"reflect"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

const anyApplicationFinalizerName = "anyapplication.finalizers.hiro.io"

// AnyApplicationReconciler reconciles a AnyApplication object
type AnyApplicationReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Config      *config.ApplicationRuntimeConfig
	SyncManager types.SyncManager
	Jobs        types.AsyncJobs
	Reconciler  reconciler.Reconciler
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete
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

	if !resource.DeletionTimestamp.IsZero() {
		// The resource is being deleted
		return r.resourceCleanup(ctx, resource)
	}

	if !containsString(resource.Finalizers, anyApplicationFinalizerName) {
		return r.addFinalizer(ctx, resource)
	}

	globalApplication, err := r.SyncManager.LoadApplication(resource)
	if err != nil {
		log.Error(err, "failed to load application state")
		return ctrl.Result{}, err
	}

	result := r.Reconciler.DoReconcile(globalApplication)

	log.Info("reconciler", "result", result)
	if result.Status.IsPresent() {
		stopRetrying := atomic.Bool{}
		newStatus := result.Status.OrEmpty()

		statusUpdater := status.NewStatusUpdater(ctx, log.WithName("Controller StatusUpdater"), r.Client, req.NamespacedName)
		err = statusUpdater.UpdateStatus(&stopRetrying, func(applicationStatus *dcpv1.AnyApplicationStatus) bool {
			return mergeStatus(applicationStatus, &newStatus)
		})
		if err != nil {
			log.Error(err, "failed to update status")
			return ctrl.Result{}, err // TODO maybe requeue
		}
	}

	result.JobsToAdd.ForEach(func(newJob types.AsyncJob) {
		applicationId := newJob.GetJobID().ApplicationId
		r.Jobs.Stop(applicationId)
		log.Info("Starting job", "jobId", newJob.GetJobID())
		r.Jobs.Execute(newJob)
	})

	return ctrl.Result{}, nil
}

func (r *AnyApplicationReconciler) addFinalizer(ctx context.Context, resource *dcpv1.AnyApplication) (ctrl.Result, error) {
	resource.Finalizers = append(resource.Finalizers, anyApplicationFinalizerName)
	if err := r.Update(ctx, resource); err != nil {
		return ctrl.Result{}, err
	}
	// Requeue after updating the finalizer
	return ctrl.Result{Requeue: true}, nil
}

func (r *AnyApplicationReconciler) resourceCleanup(ctx context.Context, resource *dcpv1.AnyApplication) (ctrl.Result, error) {
	if containsString(resource.Finalizers, anyApplicationFinalizerName) {
		// Perform cleanup logic here
		applicationId := types.ApplicationId{
			Name:      resource.Name,
			Namespace: resource.Namespace,
		}
		r.Jobs.Stop(applicationId)
		if _, err := r.SyncManager.Delete(ctx, resource); err != nil {
			return ctrl.Result{}, err
		}

		// Remove finalizer and update
		resource.Finalizers = removeString(resource.Finalizers, anyApplicationFinalizerName)
		if err := r.Update(ctx, resource); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnyApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dcpv1.AnyApplication{}).
		Named("anyapplication").
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func mergeStatus(currentStatus *dcpv1.AnyApplicationStatus, newStatus *dcpv1.AnyApplicationStatus) bool {
	updated := false
	if newStatus.Placements != nil && !reflect.DeepEqual(currentStatus.Placements, newStatus.Placements) {
		currentStatus.Placements = newStatus.Placements
		updated = true
	}
	if newStatus.State != dcpv1.UnknownGlobalState && currentStatus.State != newStatus.State {
		currentStatus.State = newStatus.State
		updated = true
	}
	if newStatus.Owner != "" && currentStatus.Owner != newStatus.Owner {
		currentStatus.Owner = newStatus.Owner
		updated = true
	}

	if newStatus.Conditions != nil {
		for _, newCondition := range newStatus.Conditions {
			found := false
			for i, existingCondition := range currentStatus.Conditions {
				if existingCondition.Type == newCondition.Type && existingCondition.ZoneId == newCondition.ZoneId {
					if existingCondition.LastTransitionTime.Time.Before(newCondition.LastTransitionTime.Time) {
						currentStatus.Conditions[i] = newCondition
						updated = true
					}
					found = true
				}
			}
			if !found {
				currentStatus.Conditions = append(currentStatus.Conditions, newCondition)
				updated = true
			}
		}
	}
	return updated
}
