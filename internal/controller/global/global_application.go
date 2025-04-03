package global

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/local"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GlobalApplication struct {
	locaApplication mo.Option[local.LocalApplication]
	application     *v1.AnyApplication
}

func LoadFromKubernetes(client client.Client, application *v1.AnyApplication) GlobalApplication {
	localApplication, err := local.LoadFromKubernetes(client, &application.Spec.Application)
	if err != nil {

	}
	return GlobalApplication{
		locaApplication: localApplication,
	}
}

func (g *GlobalApplication) GetNextState() GlobalState {
	return NewGlobal
}
