// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"strings"

	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ApplicationSpecParser struct {
	name      string
	namespace string
	resources []*unstructured.Unstructured
}

func NewSpecParser(name string, namespace string, resources []*unstructured.Unstructured) *ApplicationSpecParser {
	return &ApplicationSpecParser{
		name:      name,
		namespace: namespace,
		resources: resources,
	}
}

func (p *ApplicationSpecParser) Parse() (*api.ApplicationSpec, error) {
	resources := make([]api.ApplicationSpec_Resources_Item, 0)
	for _, resource := range p.resources {
		extracted, err := p.extractSpec(resource)
		if err != nil {
			return nil, err
		}
		resources = append(resources, extracted...)
	}
	spec := api.ApplicationSpec{
		Id: api.ResourceId{
			Name:      p.name,
			Namespace: p.namespace,
		},
		Resources: resources,
	}
	return &spec, nil
}

func (p *ApplicationSpecParser) extractSpec(u *unstructured.Unstructured) ([]api.ApplicationSpec_Resources_Item, error) {
	kind := u.GetKind()
	resourceItems := make([]api.ApplicationSpec_Resources_Item, 0)
	switch strings.ToLower(kind) {
	case "pvc":
		est := NewPVCParser()
		pvcResources, err := est.Parse(u)
		if err != nil {
			return nil, err
		}
		item := api.ApplicationSpec_Resources_Item{}
		if err := item.FromPVCResources(*pvcResources); err != nil {
			return nil, err
		}
		resourceItems = append(resourceItems, item)
	case "deployment", "statefulset", "job", "daemonset":
		est := NewWorkloadParser()
		podResources, pvcResources, err := est.Parse(u)
		if err != nil {
			return nil, err
		}
		if podResources != nil {
			item := api.ApplicationSpec_Resources_Item{}
			if err := item.FromPodResources(*podResources); err != nil {
				return nil, err
			}
			resourceItems = append(resourceItems, item)
		}
		for _, pvcResource := range pvcResources {
			item := api.ApplicationSpec_Resources_Item{}
			if err := item.FromPVCResources(pvcResource); err != nil {
				return nil, err
			}
			resourceItems = append(resourceItems, item)
		}

	default:

	}
	return resourceItems, nil
}
