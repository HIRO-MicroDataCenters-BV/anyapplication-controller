package reconciler

import (
	"context"

	"github.com/pkg/errors"
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
	globalApplication, err := global.LoadCurrentState(b.ctx, b.client, b.resource, b.config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create recondiler")
	}
	reconciler := NewReconciler(globalApplication, nil)
	return &reconciler, nil
}
