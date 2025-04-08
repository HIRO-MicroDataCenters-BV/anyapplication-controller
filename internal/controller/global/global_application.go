package global

import (
	"context"

	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/local"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type GlobalApplication struct {
	locaApplication mo.Option[local.LocalApplication]
	application     *v1.AnyApplication
}

func LoadFromKubernetes(ctx context.Context, client client.Client, application *v1.AnyApplication) GlobalApplication {
	localApplication, err := local.LoadFromKubernetes(ctx, client, &application.Spec.Application)
	if err != nil {
		log.Log.Info("error loading from kubernetes")
	}
	return GlobalApplication{
		locaApplication: localApplication,
		application:     application,
	}
}

func (g *GlobalApplication) GetNextState() GlobalState {
	return NewGlobal
}
