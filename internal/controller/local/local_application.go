package local

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalApplication struct {
	bundle   *ApplicationBundle
	status   health.HealthStatusCode
	messages []string
}

func LoadFromKubernetes(ctx context.Context, client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (mo.Option[LocalApplication], error) {
	bundle, err := LoadApplicationBundle(ctx, client, applicationSpec)
	if err != nil {
		return mo.None[LocalApplication](), err
	}
	status, messages, err := bundle.DetermineState()
	if err != nil {
		return mo.None[LocalApplication](), err
	}
	return mo.Some(LocalApplication{
		bundle:   &bundle,
		status:   status,
		messages: messages,
	}), nil
}

func (l *LocalApplication) GetApplicationStatus() health.HealthStatusCode {
	return l.status
}

func (l *LocalApplication) GetApplicationMessages() []string {
	return l.messages
}
