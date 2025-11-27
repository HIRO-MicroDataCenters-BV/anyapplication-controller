// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"encoding/json"
	"log"

	health "github.com/argoproj/gitops-engine/pkg/health"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type ApplicationBundle struct {
	availableResources []*unstructured.Unstructured
	expectedResources  []*unstructured.Unstructured
	log                logr.Logger
}

func LoadApplicationBundle(
	availableResources []*unstructured.Unstructured,
	expectedResources []*unstructured.Unstructured,
	log logr.Logger,
) (ApplicationBundle, error) {
	return ApplicationBundle{
		availableResources: availableResources,
		expectedResources:  expectedResources,
		log:                log,
	}, nil
}

func Deserialize(data string) (ApplicationBundle, error) {
	var bundle ApplicationBundle
	err := json.Unmarshal([]byte(data), &bundle)
	if err != nil {
		log.Fatal(err)
	}
	return bundle, nil
}

// UnmarshalJSON provides custom unmarshaling for ApplicationBundle.
func (bundle *ApplicationBundle) UnmarshalJSON(data []byte) error {
	type Alias ApplicationBundle
	aux := &struct {
		AvailableResources []unstructured.Unstructured `json:"availableResources"`
		ExpectedResources  []unstructured.Unstructured `json:"expectedResources"`
		*Alias
	}{
		Alias: (*Alias)(bundle),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	bundle.availableResources = Map(aux.AvailableResources, func(r unstructured.Unstructured) *unstructured.Unstructured {
		return &r
	})
	bundle.expectedResources = Map(aux.ExpectedResources, func(r unstructured.Unstructured) *unstructured.Unstructured {
		return &r
	})
	return nil
}

func (bundle *ApplicationBundle) Serialize() (string, error) {
	jsonData, err := json.Marshal(bundle)
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonData), nil
}

// MarshalJSON provides custom marshaling for ApplicationBundle.
func (bundle ApplicationBundle) MarshalJSON() ([]byte, error) {
	type Alias ApplicationBundle
	return json.Marshal(&struct {
		AvailableResources []unstructured.Unstructured `json:"availableResources"`
		ExpectedResources  []unstructured.Unstructured `json:"expectedResources"`
		*Alias
	}{
		AvailableResources: Map(bundle.availableResources, func(r *unstructured.Unstructured) unstructured.Unstructured {
			return *r
		}),
		ExpectedResources: Map(bundle.expectedResources, func(r *unstructured.Unstructured) unstructured.Unstructured {
			return *r
		}),
		Alias: (*Alias)(&bundle),
	})
}

func Map[T any, R any](slice []T, f func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func (bundle *ApplicationBundle) IsDeployed() bool {
	// Check if all expected resources are present in the available resources
	availableResourceMap := bundle.toResourceMap(bundle.availableResources)
	for _, expectedItem := range bundle.expectedResources {
		gvk := expectedItem.GetObjectKind().GroupVersionKind()
		name := types.NamespacedName{
			Namespace: expectedItem.GetNamespace(),
			Name:      expectedItem.GetName(),
		}
		if _, exists := availableResourceMap[gvk][name]; !exists {
			bundle.log.Info("Resource is missing", "gvk", gvk.String(), "name", name)
			return false // Resource is missing
		}
	}
	return true // All expected resources are present
}

func (bundle *ApplicationBundle) DetermineState() (health.HealthStatusCode, []string, error) {
	availableResourceMap := bundle.toResourceMap(bundle.availableResources)
	resourceStatuses, err := foldLeft(bundle.expectedResources, make([]health.HealthStatus, 0),
		func(acc []health.HealthStatus, expectedItem *unstructured.Unstructured) ([]health.HealthStatus, error) {
			gvk := expectedItem.GetObjectKind().GroupVersionKind()
			name := types.NamespacedName{
				Namespace: expectedItem.GetNamespace(),
				Name:      expectedItem.GetName(),
			}
			item, exists := availableResourceMap[gvk][name]
			if !exists {
				// Resource is missing, return an error status
				acc = append(acc, health.HealthStatus{
					Status:  health.HealthStatusMissing,
					Message: "Resource is missing: " + gvk.String() + " " + name.String(),
				})
				return acc, nil
			}
			status, err := determineResourceState(item)
			if err != nil {
				return acc, err
			}
			if status != nil {
				acc = append(acc, *status)
			}
			return acc, nil
		})

	if err != nil {
		return health.HealthStatusUnknown, nil, err
	}

	healthStatus, _ := foldLeft(resourceStatuses, health.HealthStatusHealthy,
		func(acc health.HealthStatusCode, item health.HealthStatus) (health.HealthStatusCode, error) {
			if health.IsWorse(acc, item.Status) {
				acc = item.Status
			}
			return acc, nil
		},
	)

	messages, _ := foldLeft(resourceStatuses, make([]string, 0),
		func(acc []string, item health.HealthStatus) ([]string, error) {
			if item.Message != "" {
				acc = append(acc, item.Message)
			}
			return acc, nil
		},
	)

	return healthStatus, messages, nil
}

func (bundle *ApplicationBundle) toResourceMap(
	availableResources []*unstructured.Unstructured,
) map[schema.GroupVersionKind]map[types.NamespacedName]*unstructured.Unstructured {

	availableResourceMap := make(map[schema.GroupVersionKind]map[types.NamespacedName]*unstructured.Unstructured)
	for _, resource := range availableResources {
		gvk := resource.GetObjectKind().GroupVersionKind()
		name := types.NamespacedName{
			Namespace: resource.GetNamespace(),
			Name:      resource.GetName(),
		}
		if _, exists := availableResourceMap[gvk]; !exists {
			availableResourceMap[gvk] = make(map[types.NamespacedName]*unstructured.Unstructured)
		}
		if _, exists := availableResourceMap[gvk][name]; exists {
			bundle.log.Info("Duplicate resource found", "gvk", gvk.String(), "name", name)
			continue // Skip duplicates
		}
		availableResourceMap[gvk][name] = resource
	}
	return availableResourceMap
}

func determineResourceState(resource *unstructured.Unstructured) (*health.HealthStatus, error) {
	status, err := health.GetResourceHealth(resource, nil)
	return status, err
}

func foldLeft[T, R any](arr []T, initial R, fn func(acc R, item T) (R, error)) (R, error) {
	acc := initial
	for _, item := range arr {
		var err error
		acc, err = fn(acc, item)
		if err != nil {
			return acc, err // Return the accumulator and the error if one occurred
		}
	}
	return acc, nil
}
