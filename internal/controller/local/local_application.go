package local

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type LocalApplication struct {
	bundle   *ApplicationBundle
	config   *config.ApplicationRuntimeConfig
	status   health.HealthStatusCode
	messages []string
	version  string
	clock    clock.Clock
}

func NewFromUnstructured(
	version string,
	availableResources []*unstructured.Unstructured,
	expectedResources []*unstructured.Unstructured,
	config *config.ApplicationRuntimeConfig,
	clock clock.Clock,
) (mo.Option[LocalApplication], error) {
	if len(availableResources) == 0 {
		return mo.None[LocalApplication](), nil
	}
	bundle, err := LoadApplicationBundle(availableResources, expectedResources)
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
		version:  version,
		clock:    clock,
	}), nil
}

func (l *LocalApplication) GetStatus() health.HealthStatusCode {
	return l.status
}

func (l *LocalApplication) IsDeployed() bool {
	return l.bundle.IsDeployed()
}

func (l *LocalApplication) GetMessages() []string {
	return l.messages
}

func (l *LocalApplication) GetCondition() v1.ConditionStatus {
	condition := v1.ConditionStatus{
		Type:               v1.LocalConditionType,
		ZoneId:             l.config.ZoneId,
		LastTransitionTime: l.clock.NowTime(),
		Status:             string(l.status),
		Reason:             "",
		Msg:                "",
	}
	return condition
}

func FakeLocalApplication(
	config *config.ApplicationRuntimeConfig,
	clock clock.Clock,
) LocalApplication {
	return LocalApplication{
		bundle:   &ApplicationBundle{},
		status:   health.HealthStatusProgressing,
		messages: []string{},
		config:   config,
		version:  "1",
		clock:    clock,
	}
}
