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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

const anyApplicationFinalizerName = "finalizers.dcp.hiro.io/anyapplication"

// AnyApplicationReconciler reconciles a AnyApplication object
type AnyApplicationReconciler struct {
	client.Client
	Recorder     record.EventRecorder
	Scheme       *runtime.Scheme
	Config       *config.ApplicationRuntimeConfig
	Applications types.Applications
	Jobs         types.AsyncJobs
	Reconciler   reconciler.Reconciler
	Log          logr.Logger
	Events       *events.Events
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dcp.hiro.io,resources=anyapplications/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *AnyApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconcile method")
	resource := &dcpv1.AnyApplication{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("AnyApplication resource not found. Ignoring since object must be deleted", "name", req.Name, "namespace", req.Namespace)
			return reconcile.Result{}, nil
		}
		r.Log.Error(err, "Unable to get AnyApplication ", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	if resource.Status.Owner == "" {
		return r.InitializeState(ctx, resource.GetNamespacedName())
	}

	if !resource.DeletionTimestamp.IsZero() {
		// The resource is being deleted
		return r.resourceCleanup(ctx, resource)
	}

	if !containsString(resource.Finalizers, anyApplicationFinalizerName) {
		return r.addFinalizer(ctx, resource)
	}

	globalApplication, err := r.Applications.LoadApplication(resource)
	if err != nil {
		r.Log.Error(err, "failed to load application state")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 1 * time.Second,
		}, err
	}

	shouldHandle := globalApplication.IsDeployed() ||
		isNewApplication(resource) ||
		globalApplication.HasZoneStatus() ||
		isOwnerOrPlacementZone(resource, r.Config.ZoneId)

	if !shouldHandle {
		return ctrl.Result{}, nil
	}

	r.Log.Info("reconciler", "initial status", resource.Status)

	result := r.Reconciler.DoReconcile(globalApplication)

	r.Log.Info("reconciler", "result status", result)

	result.JobsToAdd.ForEach(func(newJob types.AsyncJob) {
		applicationId := newJob.GetJobID().ApplicationId
		currentJobOpt := r.Jobs.GetCurrent(applicationId)
		currentJob, jobIsRunning := currentJobOpt.Get()
		differentJobIsRunning := jobIsRunning && currentJob.GetJobID() != newJob.GetJobID()

		if jobIsRunning && differentJobIsRunning {
			r.Log.Info("Stopping job", "jobId", currentJob.GetJobID())
			r.Jobs.Stop(applicationId)
		}
	})

	ctrlResult := ctrl.Result{}
	err = nil

	if result.Status.IsPresent() {
		newStatus := result.Status.OrEmpty()

		statusUpdater := status.NewStatusUpdater(
			ctx,
			r.Log.WithName("Controller StatusUpdater"),
			r.Client,
			req.NamespacedName,
			r.Config.ZoneId,
			r.Events,
		)

		err = statusUpdater.UpdateStatus(func(applicationStatus *dcpv1.AnyApplicationStatus, zoneId string) (bool, events.Event) {
			return mergeStatus(applicationStatus, &newStatus, zoneId)
		})
		if err != nil {
			r.Log.Error(err, "failed to update status: requeuing")
			ctrlResult = ctrl.Result{
				Requeue:      true,
				RequeueAfter: 1 * time.Second,
			}

		}
	}

	result.JobsToAdd.ForEach(func(newJob types.AsyncJob) {
		applicationId := newJob.GetJobID().ApplicationId
		currentJobOpt := r.Jobs.GetCurrent(applicationId)
		currentJob, jobIsRunning := currentJobOpt.Get()
		theSameJobIsRunning := jobIsRunning && currentJob.GetJobID() == newJob.GetJobID()

		if !theSameJobIsRunning {
			r.Log.Info("Starting job", "jobId", newJob.GetJobID())
			r.Jobs.Execute(newJob)
		}
	})

	return ctrlResult, err
}

func (r *AnyApplicationReconciler) InitializeState(ctx context.Context, resourceName client.ObjectKey) (ctrl.Result, error) {
	statusUpdater := status.NewStatusUpdater(
		ctx,
		r.Log.WithName("Controller StatusUpdater"),
		r.Client,
		resourceName,
		r.Config.ZoneId,
		r.Events,
	)

	err := statusUpdater.UpdateStatus(func(applicationStatus *dcpv1.AnyApplicationStatus, zoneId string) (bool, events.Event) {
		if applicationStatus.State == "" {
			applicationStatus.Owner = r.Config.ZoneId
			applicationStatus.State = dcpv1.NewGlobalState
			event := events.Event{
				Reason: events.GlobalStateChangeReason,
				Msg:    "Owner set to " + r.Config.ZoneId + ". Global State set to " + string(dcpv1.NewGlobalState),
			}
			return true, event
		}
		return false, events.Event{}
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
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
		if _, err := r.Applications.Delete(ctx, resource); err != nil {
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

func mergeStatus(currentStatus *dcpv1.AnyApplicationStatus, newStatus *dcpv1.AnyApplicationStatus, zone string) (bool, events.Event) {
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

	zoneStatus, currentExists := currentStatus.GetStatusFor(zone)
	newZoneStatus, exists := newStatus.GetStatusFor(zone)

	if exists {
		if !currentExists {
			zoneStatus = currentStatus.GetOrCreateStatusFor(zone)
		}
		if newZoneStatus.Conditions != nil {
			for _, newCondition := range newZoneStatus.Conditions {
				found := false
				for i, existingCondition := range zoneStatus.Conditions {
					if existingCondition.Type == newCondition.Type && existingCondition.ZoneId == newCondition.ZoneId {
						if existingCondition.LastTransitionTime.Time.Before(newCondition.LastTransitionTime.Time) {
							if existingCondition.Status != newCondition.Status {
								zoneStatus.Conditions[i] = newCondition
								updated = true
							}
						}
						found = true
					}
				}
				if !found {
					zoneStatus.Conditions = append(zoneStatus.Conditions, newCondition)
					updated = true
				}
			}
		}
	} else {
		currentStatus.RemoveZone(zone)
	}
	event := events.Event{Reason: reason, Msg: msg}
	return updated, event
}

func isOwnerOrPlacementZone(resource *dcpv1.AnyApplication, zone string) bool {
	isOwnerZone := resource.Status.Owner == zone
	isPlacementZone := false
	for _, placement := range resource.Status.Placements {
		if placement.Zone == zone {
			isPlacementZone = true
		}
	}

	return isOwnerZone || isPlacementZone
}

func isNewApplication(resource *dcpv1.AnyApplication) bool {
	return resource.Status.State == ""
}
