package local

import (
	"encoding/json"
	"log"

	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationBundle struct {
	Deployments   *unstructured.UnstructuredList
	StatefullSets *unstructured.UnstructuredList
	Jobs          *unstructured.UnstructuredList
	DaemonSets    *unstructured.UnstructuredList
}

func Load(client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (ApplicationBundle, error) {
	// deployments_list := &unstructured.UnstructuredList{}
	// for _, selectorSpec := range applicationSpec.ResourceSelector {
	// 	deployments, err := LoadResource(client, applicationSpec, "Deployment", selectorSpec.Namespace)
	// }

	return ApplicationBundle{}, nil
}

func LoadResource(client client.Client, applicationSpec *v1.ApplicationMatcherSpec, resourceType string, namespace string) (unstructured.UnstructuredList, error) {
	deployments_list := unstructured.UnstructuredList{}
	deployments_list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DeploymentList",
		Version: "v1",
	})
	// _ = client.List(context.Background(), &deployments_list, &client.ListOptions{
	// 	Namespace: namespace,
	// })

	return deployments_list, nil
}

func Deserialize(data string) (ApplicationBundle, error) {
	var bundle ApplicationBundle
	err := json.Unmarshal([]byte(data), &bundle)
	if err != nil {
		log.Fatal(err)
	}
	return bundle, nil
}

func (bundle *ApplicationBundle) Serialize() (string, error) {

	jsonData, err := json.Marshal(bundle)
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonData), nil
}
