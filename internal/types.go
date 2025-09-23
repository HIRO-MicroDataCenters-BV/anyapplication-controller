package internal

import (
	"context"

	"hiro.io/anyapplication/internal/httpapi/api"
)

type ApplicationSpecs interface {
	GetApplicationSpec(ctx context.Context, namespace string, name string) (*api.ApplicationSpec, error)
}
