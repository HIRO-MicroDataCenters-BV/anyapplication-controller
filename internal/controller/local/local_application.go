package local

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalApplication struct {
	// deployments *unstructured.UnstructuredList{}
}

func LoadFromKubernetes(client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (mo.Option[LocalApplication], error) {
	deployments_list := &unstructured.UnstructuredList{}
	deployments_list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DeploymentList",
		Version: "v1",
	})
	// _ = client.List(context.Background(), deployments_list, &client.ListOptions{
	// 	Namespace: "default",
	// })

	return mo.Some(LocalApplication{}), nil
}

func (l *LocalApplication) GetCurrentState() LocalState {
	if l.isNew() {
		return NewLocal
	} else if l.isStarting() {
		return Starting
	} else if l.isRunning() {
		return Running
	} else if l.isCompleted() {
		return Completed
	} else {
		return UnknownLocal
	}

}

func (l *LocalApplication) isCompleted() bool {
	return false
}

func (l *LocalApplication) isRunning() bool {
	return false
}

func (l *LocalApplication) isStarting() bool {
	return false
}

func (l *LocalApplication) isNew() bool {
	return false
}
