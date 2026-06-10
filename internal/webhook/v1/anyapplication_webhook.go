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

package v1

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller"
)

// SetupAnyApplicationWebhookWithManager registers the webhook for AnyApplication in the manager.
func SetupAnyApplicationWebhookWithManager(mgr ctrl.Manager, log logr.Logger, currentZoneId string) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&dcpv1.AnyApplication{}).
		WithValidator(&AnyApplicationCustomValidator{currentZoneId: currentZoneId, log: log}).
		WithDefaulter(&AnyApplicationCustomDefaulter{log: log}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-dcp-hiro-io-v1-anyapplication,mutating=true,failurePolicy=fail,sideEffects=None,groups=dcp.hiro.io,resources=anyapplications,verbs=create;update,versions=v1,name=manyapplication-v1.kb.io,admissionReviewVersions=v1

// AnyApplicationCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind AnyApplication when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AnyApplicationCustomDefaulter struct {
	log logr.Logger
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind AnyApplication.
func (d *AnyApplicationCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	anyapplication, ok := obj.(*dcpv1.AnyApplication)

	if !ok {
		return fmt.Errorf("expected an AnyApplication object but got %T", obj)
	}
	d.log.Info("Defaulting for AnyApplication", "name", anyapplication.GetName())
	if !slices.Contains(anyapplication.Finalizers, controller.AnyApplicationFinalizerName) {
		d.log.Info("Adding... ", "finalizer", controller.AnyApplicationFinalizerName)
		anyapplication.Finalizers = append(anyapplication.Finalizers, controller.AnyApplicationFinalizerName)
	}
	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dcp-hiro-io-v1-anyapplication,mutating=false,failurePolicy=fail,sideEffects=None,groups=dcp.hiro.io,resources=anyapplications,verbs=create;update;delete,versions=v1,name=vanyapplication-v1.kb.io,admissionReviewVersions=v1

// AnyApplicationCustomValidator struct is responsible for validating the AnyApplication resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AnyApplicationCustomValidator struct {
	currentZoneId string
	log           logr.Logger
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type AnyApplication.
func (v *AnyApplicationCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type AnyApplication.
func (v *AnyApplicationCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type AnyApplication.
func (v *AnyApplicationCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	anyapplication, ok := obj.(*dcpv1.AnyApplication)
	if !ok {
		return nil, fmt.Errorf("expected a AnyApplication object but got %T", obj)
	}
	v.log.Info("Validation for AnyApplication upon deletion", "name", anyapplication.GetName())

	isOwnerZone := anyapplication.Status.Ownership.Owner == v.currentZoneId
	isRemovingState := anyapplication.Status.Ownership.State == dcpv1.RemovingGlobalState
	if !isOwnerZone {
		if !isRemovingState {
			v.log.Info("Cannot remove from non owner zone not in removing state", "name", anyapplication.GetName())
			return nil, fmt.Errorf("cannot remove from non owner zone not in removing state %T", obj)
		}
	}

	return nil, nil
}
