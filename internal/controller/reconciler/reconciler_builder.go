package reconciler

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/global"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerBuilder struct {
	client   client.Client
	resource *v1.AnyApplication
}

func NewReconcilerBuilder(client client.Client, resource *v1.AnyApplication) ReconcilerBuilder {
	return ReconcilerBuilder{
		client:   client,
		resource: resource,
	}
}

func (b ReconcilerBuilder) Build() (*Reconciler, error) {
	globalApplication := global.LoadFromKubernetes(b.client, b.resource)
	reconciler := NewReconciler(globalApplication)
	return &reconciler, nil
}
