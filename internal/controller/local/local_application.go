package local

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationRuntimeOptions struct {
	ZoneId string
}

type LocalApplication struct {
	bundle         *ApplicationBundle
	runtimeOptions ApplicationRuntimeOptions
	status         health.HealthStatusCode
	messages       []string
}

func LoadCurrentState(
	ctx context.Context,
	client client.Client,
	applicationSpec *v1.ApplicationMatcherSpec,
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
	}), nil
}

func (l *LocalApplication) GetStatus() health.HealthStatusCode {
	return l.status
}

func (l *LocalApplication) GetMessages() []string {
	return l.messages
}

func (l *LocalApplication) GetCondition() v1.ConditionStatus {
	condition := v1.ConditionStatus{
		Type:               string(LocalStatusType),
		ZoneId:             l.runtimeOptions.ZoneId,
		LastTransitionTime: metav1.Now(),
		Reason:             "",
		Msg:                "",
	}
	return condition
}
