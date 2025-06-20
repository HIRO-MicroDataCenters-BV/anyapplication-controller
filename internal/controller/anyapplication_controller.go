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
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

const anyApplicationFinalizerName = "anyapplication.finalizers.hiro.io"

// AnyApplicationReconciler reconciles a AnyApplication object
type AnyApplicationReconciler struct {
	client.Client
	Recorder    record.EventRecorder
	Scheme      *runtime.Scheme
	Config      *config.ApplicationRuntimeConfig
	SyncManager types.SyncManager
	Jobs        types.AsyncJobs
	Reconciler  reconciler.Reconciler
	Log         logr.Logger
	Events      *events.Events
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *AnyApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	resource := &dcpv1.AnyApplication{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		r.Log.Error(err, "Unable to get AnyApplication ", "name", req.Name, "namespace", req.Namespace)
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
		r.Log.Error(err, "failed to load application state")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 1 * time.Second,
		}, err
	}

	if !globalApplication.IsDeployed() && !currentZone(resource, r.Config.ZoneId) {
		return ctrl.Result{}, nil
	}

	result := r.Reconciler.DoReconcile(globalApplication)

	r.Log.Info("reconciler result", result)
	if result.Status.IsPresent() {
		stopRetrying := atomic.Bool{}
		newStatus := result.Status.OrEmpty()

		statusUpdater := status.NewStatusUpdater(
			ctx,
			r.Log.WithName("Controller StatusUpdater"),
			r.Client,
			req.NamespacedName,
			r.Config.ZoneId,
			r.Events,
		)

		err = statusUpdater.UpdateStatus(&stopRetrying, func(applicationStatus *dcpv1.AnyApplicationStatus) (bool, events.Event) {
			return mergeStatus(applicationStatus, &newStatus)
		})
		if err != nil {
			r.Log.Error(err, "failed to update status: requeuing")
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 1 * time.Second,
			}, err
		}
	}

	result.JobsToAdd.ForEach(func(newJob types.AsyncJob) {
		applicationId := newJob.GetJobID().ApplicationId
		r.Jobs.Stop(applicationId)
		r.Log.Info("Starting job", "jobId", newJob.GetJobID())
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

func mergeStatus(currentStatus *dcpv1.AnyApplicationStatus, newStatus *dcpv1.AnyApplicationStatus) (bool, events.Event) {
	updated := false
	reason := events.GlobalStateChangeReason
	msg := ""
	if newStatus.Placements != nil && !reflect.DeepEqual(currentStatus.Placements, newStatus.Placements) {
		currentStatus.Placements = newStatus.Placements
		msg += fmt.Sprintf("Placements are set to '%v'. ", newStatus.Placements)
		updated = true
	}
	if newStatus.State != dcpv1.UnknownGlobalState && currentStatus.State != newStatus.State {
		currentStatus.State = newStatus.State
		msg += "Global state changed to '" + string(newStatus.State) + "'. "
		updated = true
	}
	if newStatus.Owner != "" && currentStatus.Owner != newStatus.Owner {
		currentStatus.Owner = newStatus.Owner
		msg += "Owner changed to '" + newStatus.Owner + "'."
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
	event := events.Event{Reason: reason, Msg: msg}
	return updated, event
}

func currentZone(resource *dcpv1.AnyApplication, zone string) bool {

	isNewApplication := resource.Status.State == ""

	isOwnerZone := resource.Status.Owner == zone
	isPlacementZone := false
	for _, placement := range resource.Status.Placements {
		if placement.Zone == zone {
			isPlacementZone = true
		}
	}

	return isNewApplication || isOwnerZone || isPlacementZone
}
