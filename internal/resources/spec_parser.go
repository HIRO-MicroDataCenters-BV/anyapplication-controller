// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"github.com/samber/mo"
	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ApplicationSpecParser struct {
	name      string
	namespace string
	resources []unstructured.Unstructured
}

func NewParser(name string, namespace string, resources []unstructured.Unstructured) *ApplicationSpecParser {
	return &ApplicationSpecParser{
		name:      name,
		namespace: namespace,
		resources: resources,
	}
}

func (p *ApplicationSpecParser) ParseResources() (*api.ApplicationSpec, error) {
	resources := make([]api.ApplicationSpec_Resources_Item, 0)
	for _, resource := range p.resources {
		p.extractSpec(&resource)
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

func (p *ApplicationSpecParser) extractSpec(resource *unstructured.Unstructured) (mo.Option[[]api.ApplicationSpec_Resources_Item], error) {
	kind := resource.GetKind()
	switch kind {
	case "pvc":
	case "deployment", "statefulset", "job", "daemonset":

	default:

	}
	return mo.None[[]api.ApplicationSpec_Resources_Item](), nil
}
