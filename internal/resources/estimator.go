// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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

type ResourceEstimator struct{}

func NewResourceEstimator() *ResourceEstimator {
	return &ResourceEstimator{}
}

func (re *ResourceEstimator) EstimateFromYAML(renderedYAML string) (ResourceTotals, error) {
	objects, err := re.parseYAMLDocs(renderedYAML)
	if err != nil {
		return ResourceTotals{}, err
	}

	total := ResourceTotals{}

	for _, obj := range objects {
		res, err := re.extractResources(obj)
		if err != nil {
			return ResourceTotals{}, err
		}
		total.Requests.CPU.Add(res.Requests.CPU)
		total.Requests.Memory.Add(res.Requests.Memory)
		total.Requests.EphemeralStorage.Add(res.Requests.EphemeralStorage)

		total.Limits.CPU.Add(res.Limits.CPU)
		total.Limits.Memory.Add(res.Limits.Memory)
		total.Limits.EphemeralStorage.Add(res.Limits.EphemeralStorage)
	}

	return total, nil
}

func (re *ResourceEstimator) parseYAMLDocs(yamlText string) ([]*unstructured.Unstructured, error) {
	docs := strings.Split(yamlText, "\n---")
	objects := make([]*unstructured.Unstructured, 0, len(docs))

	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			continue
		}

		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(trimmed), u)
		if err != nil {
			return nil, err
		}

		objects = append(objects, u)
	}

	return objects, nil
}

func (re *ResourceEstimator) extractResources(obj *unstructured.Unstructured) (ResourceTotals, error) {
	kind := obj.GetKind()
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")

	templateSpec := spec
	if tmpl, ok := spec["template"]; ok {
		if tmplMap, ok := tmpl.(map[string]interface{}); ok {
			templateSpec, _, _ = unstructured.NestedMap(tmplMap, "spec")
		}
	}

	containers, _, err := unstructured.NestedSlice(templateSpec, "containers")
	if err != nil {
		return ResourceTotals{}, err
	}
	initContainers, _, err := unstructured.NestedSlice(templateSpec, "initContainers")
	if err != nil {
		return ResourceTotals{}, err
	}

	replicas := int64(1)
	if kind == "Deployment" || kind == "StatefulSet" {
		if r, found, _ := unstructured.NestedInt64(spec, "replicas"); found {
			replicas = r
		}
	}

	totals := ResourceTotals{}
	if err := re.collectResources(containers, replicas, &totals); err != nil {
		return ResourceTotals{}, err
	}
	// init containers run once
	if err := re.collectResources(initContainers, 1, &totals); err != nil {
		return ResourceTotals{}, err
	}

	return totals, nil
}

func (re *ResourceEstimator) collectResources(containers []interface{}, replicas int64, totals *ResourceTotals) error {
	if containers == nil {
		return nil
	}
	for _, c := range containers {
		if cMap, ok := c.(map[string]interface{}); ok {
			if resMap, found, _ := unstructured.NestedMap(cMap, "resources"); found {
				err := re.addResources(&totals.Requests, resMap["requests"], replicas)
				if err != nil {
					return err
				}
				err = re.addResources(&totals.Limits, resMap["limits"], replicas)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (re *ResourceEstimator) addResources(target *ResourceValues, input interface{}, replicas int64) error {
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

		switch k {
		case "cpu":
			target.CPU.Add(q)
		case "memory":
			target.Memory.Add(q)
		case "ephemeral-storage":
			target.EphemeralStorage.Add(q)
		}
	}
	return nil
}
