package resources

import (
	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type PVCParser struct{}

func NewPVCParser() *PVCParser {
	return &PVCParser{}
}

func (p *PVCParser) Parse(obj *unstructured.Unstructured) (*api.PVCResources, error) {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	storageClass := getNestedString(spec, "storageClassName")
	requests := map[string]*resource.Quantity{}
	limits := map[string]*resource.Quantity{}
	if err := CollectResources([]interface{}{spec}, 1, &requests, &limits); err != nil {
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

	totals := api.PVCResources{
		Id:           api.ResourceId{Name: name, Namespace: namespace},
		Limits:       Limits,
		Replica:      1,
		Requests:     Requests,
		StorageClass: storageClass,
	}
	return &totals, nil

}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructured.NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return val
}
