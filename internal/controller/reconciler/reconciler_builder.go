package reconciler

import (
	"context"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/global"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerBuilder struct {
	ctx      context.Context
	client   client.Client
	resource *v1.AnyApplication
	config   *config.ApplicationRuntimeConfig
}

func NewReconcilerBuilder(ctx context.Context, client client.Client, resource *v1.AnyApplication, config *config.ApplicationRuntimeConfig) ReconcilerBuilder {
	return ReconcilerBuilder{
		ctx:      ctx,
		client:   client,
		resource: resource,
		config:   config,
	}
}

func (b ReconcilerBuilder) Build() (*Reconciler, error) {
	globalApplication := global.LoadCurrentState(b.ctx, b.client, b.resource, b.config)
	reconciler := NewReconciler(globalApplication, nil)
	return &reconciler, nil
}
