package local

import (
	"context"

	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalApplication struct {
	bundle *ApplicationBundle
	state  LocalState
}

func LoadFromKubernetes(ctx context.Context, client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (mo.Option[LocalApplication], error) {
	bundle, err := LoadApplicationBundle(ctx, client, applicationSpec)
	if err != nil {
		return mo.None[LocalApplication](), err
	}
	return mo.Some(LocalApplication{
		bundle: &bundle,
		state:  NewLocal,
	}), nil
}

func (l *LocalApplication) GetCurrentState() LocalState {
	return l.state
}
