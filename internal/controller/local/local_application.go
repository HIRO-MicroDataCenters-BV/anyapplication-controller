package local

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/go-logr/logr"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
	log logr.Logger,
) (mo.Option[LocalApplication], error) {
	if len(availableResources) == 0 {
		return mo.None[LocalApplication](), nil
	}
	bundle, err := LoadApplicationBundle(availableResources, expectedResources, log)
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
	isDeployed bool,
) LocalApplication {

	appBundle := &ApplicationBundle{}
	if !isDeployed {

		bundle, err := LoadApplicationBundle(
			[]*unstructured.Unstructured{},
			[]*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"namespace": config.ZoneId,
							"name":      "fake-configmap",
						},
					},
				},
			},
			logf.Log,
		)
		if err != nil {
			panic("Failed to create fake LocalApplication: " + err.Error())
		}
		appBundle = &bundle
	}
	return LocalApplication{
		bundle:   appBundle,
		status:   health.HealthStatusProgressing,
		messages: []string{},
		config:   config,
		version:  "1",
		clock:    clock,
	}
}
