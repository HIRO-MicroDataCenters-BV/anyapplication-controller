package local

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalApplication struct {
	bundle   *ApplicationBundle
	config   *config.ApplicationRuntimeConfig
	status   health.HealthStatusCode
	messages []string
}

func LoadCurrentState(
	ctx context.Context,
	client client.Client,
	applicationSpec *v1.ApplicationMatcherSpec,
	config *config.ApplicationRuntimeConfig,
) (mo.Option[LocalApplication], error) {
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
		config:   config,
	}), nil
}

func NewLocalApplicationFromTemplate(template string) (mo.Option[LocalApplication], error) {
	return mo.None[LocalApplication](), nil
}

func (l *LocalApplication) GetStatus() health.HealthStatusCode {
	return l.status
}

func (l *LocalApplication) GetMessages() []string {
	return l.messages
}

func (l *LocalApplication) GetCondition() v1.ConditionStatus {
	condition := v1.ConditionStatus{
		Type:               v1.LocalConditionType,
		ZoneId:             l.config.ZoneId,
		LastTransitionTime: metav1.Now(),
		Status:             string(l.status),
		Reason:             "",
		Msg:                "",
	}
	return condition
}

func FakeLocalApplication(
	config *config.ApplicationRuntimeConfig,
) LocalApplication {
	return LocalApplication{
		bundle:   &ApplicationBundle{},
		status:   health.HealthStatusProgressing,
		messages: []string{},
		config:   config,
	}
}
