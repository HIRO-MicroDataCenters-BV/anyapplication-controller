package job

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("PlacementJob", func() {
	var (
		placementJob  *LocalPlacementJob
		kubeClient    client.Client
		helmClient    helm.FakeHelmClient
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		fakeClock     clock.Clock
		runtimeConfig config.ApplicationRuntimeConfig
		fakeEvents    events.Events
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)

		fakeEvents = events.NewFakeEvents()

		application = &v1.AnyApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: v1.AnyApplicationSpec{
				Application: v1.ApplicationMatcherSpec{
					HelmSelector: &v1.HelmSelectorSpec{
						Repository: "test-repo",
						Chart:      "test-chart",
						Version:    "1.0.0",
					},
				},
				Zones: 1,
				PlacementStrategy: v1.PlacementStrategySpec{
					Strategy: v1.PlacementStrategyLocal,
				},
				RecoverStrategy: v1.RecoverStrategySpec{},
			},
			Status: v1.AnyApplicationStatus{
				Owner: "zone",
				State: v1.PlacementGlobalState,
			},
		}

		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}

		fakeClock = clock.NewFakeClock()

		helmClient = helm.NewFakeHelmClient()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		application = application.DeepCopy()
		placementJob = NewLocalPlacementJob(application, &runtimeConfig, fakeClock, logf.Log, &fakeEvents)
	})

	It("should return initial status", func() {
		Expect(placementJob.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.PlacementConditionType,
			ZoneId:             "zone",
			ZoneVersion:        "999",
			Status:             string(v1.PlacementStatusInProgress),
			LastTransitionTime: fakeClock.NowTime(),
		},
		))
	})

	It("should run and apply done status", func() {
		context := NewAsyncJobContext(helmClient, kubeClient, context.TODO(), nil)

		placementJob.Run(context)

		result := &v1.AnyApplication{}
		_ = kubeClient.Get(context.GetGoContext(), client.ObjectKeyFromObject(application), result)

		Expect(result.Status.Conditions).To(Equal(
			[]v1.ConditionStatus{
				{
					Type:               v1.PlacementConditionType,
					ZoneId:             "zone",
					ZoneVersion:        "1000",
					Status:             string(v1.PlacementStatusDone),
					LastTransitionTime: fakeClock.NowTime(),
				},
			},
		))

		Expect(placementJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				ZoneVersion:        "999",
				Status:             string(v1.PlacementStatusDone),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

	})

})
