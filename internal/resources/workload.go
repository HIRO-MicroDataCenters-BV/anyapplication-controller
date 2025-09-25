// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"strings"

	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceValues struct {
	CPU              resource.Quantity
	Memory           resource.Quantity
	EphemeralStorage resource.Quantity
}

type ResourceTotals struct {
	Requests ResourceValues
	Limits   ResourceValues
}

type WorkloadParser struct{}

func NewWorkloadParser() *WorkloadParser {
	return &WorkloadParser{}
}

func (re *WorkloadParser) Parse(obj *unstructured.Unstructured) (*api.PodResources, []api.PVCResources, error) {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	kind := obj.GetKind()
	spec, specFound, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, nil, err
	}
	if !specFound {
		return nil, nil, nil
	}

	replicas := int32(1)
	if kind == "Deployment" || kind == "StatefulSet" {
		if r, found, _ := unstructured.NestedInt64(spec, "replicas"); found {
			replicas = int32(r)
		}
	}
	templateSpec := spec
	if tmpl, ok := spec["template"]; ok {
		if tmplMap, ok := tmpl.(map[string]interface{}); ok {
			templateSpec, _, _ = unstructured.NestedMap(tmplMap, "spec")
		}
	}
	podResources, err := CollectPodResources(templateSpec, replicas, name, namespace)
	if err != nil {
		return nil, nil, err
	}

	volumeClaimTemplateSpec := []interface{}(nil)
	if tmpl, ok := spec["volumeClaimTemplates"]; ok {
		if tmplMap, ok := tmpl.([]interface{}); ok {
			volumeClaimTemplateSpec = tmplMap
		}
	}

	pvcResources, err := CollectPVCResources(volumeClaimTemplateSpec, replicas)
	if err != nil {
		return nil, nil, err
	}

	return podResources, pvcResources, nil
}

func CollectPVCResources(volumeClaimTemplateSpecs []interface{}, replicas int32) ([]api.PVCResources, error) {
	if volumeClaimTemplateSpecs == nil {
		return nil, nil
	}
	pvcResources := make([]api.PVCResources, 0)
	for _, spec := range volumeClaimTemplateSpecs {
		if volumeClaimTemplateSpec, ok := spec.(map[string]interface{}); ok {
			est := NewPVCParser()
			parsed, err := est.ParseClaimTemplate(volumeClaimTemplateSpec)
			if err != nil {
				return nil, err
			}
			parsed.Replica = 3
			pvcResources = append(pvcResources, *parsed)
		}
	}

	return pvcResources, nil
}

func CollectPodResources(templateSpec map[string]interface{}, replicas int32, name string, namespace string) (*api.PodResources, error) {

	objectsWithResources := make([]interface{}, 0)
	containers, containersFound, err := unstructured.NestedSlice(templateSpec, "containers")
	if err != nil {
		return nil, err
	}
	if containersFound {
		objectsWithResources = append(objectsWithResources, containers...)
	}
	initContainers, initContainersFound, err := unstructured.NestedSlice(templateSpec, "initContainers")
	if err != nil {
		return nil, err
	}
	if initContainersFound {
		objectsWithResources = append(objectsWithResources, initContainers...)
	}

	requests := map[string]*resource.Quantity{}
	limits := map[string]*resource.Quantity{}
	if err := CollectResources(objectsWithResources, 1, &requests, &limits); err != nil {
		return nil, err
	}
	Limits := map[string]string{}
	for k, v := range limits {
		Limits[k] = v.String()
	}
	Requests := map[string]string{}
	for k, v := range requests {
		Requests[k] = v.String()
	}

	totals := api.PodResources{
		Id:       api.ResourceId{Name: name, Namespace: namespace},
		Limits:   Limits,
		Replica:  replicas,
		Requests: Requests,
	}
	return &totals, nil
}

func CollectResources(object []interface{}, replicas int64, totalRequests *map[string]*resource.Quantity, totalLimits *map[string]*resource.Quantity) error {
	if object == nil {
		return nil
	}

	for _, c := range object {
		if cMap, ok := c.(map[string]interface{}); ok {
			if resMap, found, _ := unstructured.NestedMap(cMap, "resources"); found {
				err := addResources(totalRequests, resMap["requests"], replicas)
				if err != nil {
					return err
				}
				err = addResources(totalLimits, resMap["limits"], replicas)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addResources(totals *map[string]*resource.Quantity, input interface{}, replicas int64) error {
	if input == nil {
		return nil
	}
	resMap, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid resource map: %v", input)
	}

	for k, v := range resMap {
		valStr, ok := v.(string)
		if !ok {
			continue
		}

		q, err := resource.ParseQuantity(valStr)
		if err != nil {
			continue
		}

		replicas := resource.MustParse(fmt.Sprintf("%d", replicas))
		replicasInt, ok := replicas.AsInt64()
		if !ok {
			return fmt.Errorf("failed to convert replicas to int64: %v", err)
		}
		q.Mul(replicasInt)

		key := strings.ToLower(k)
		current, exists := (*totals)[key]
		if !exists {
			(*totals)[key] = &q
		} else {
			current.Add(q)
		}
	}
	return nil
}
