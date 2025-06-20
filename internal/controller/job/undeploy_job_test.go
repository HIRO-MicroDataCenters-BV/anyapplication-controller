package job

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/cache"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("UndeployJob", func() {
	var (
		undeployJob   *UndeployJob
		kubeClient    client.Client
		helmClient    helm.FakeHelmClient
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		fakeClock     clock.Clock
		runtimeConfig config.ApplicationRuntimeConfig
		syncManager   types.SyncManager
		gitOpsEngine  *fixture.FakeGitOpsEngine
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
		gitOpsEngine = fixture.NewFakeGitopsEngine()
		fakeClock = clock.NewFakeClock()

		helmClient = helm.NewFakeHelmClient()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		application = application.DeepCopy()
		clusterCache := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})

		syncManager = sync.NewSyncManager(kubeClient, helmClient, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		undeployJob = NewUndeployJob(application, &runtimeConfig, fakeClock, logf.Log, &fakeEvents)
	})

	It("should return initial status", func() {
		Expect(undeployJob.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.UndeploymenConditionType,
			ZoneId:             "zone",
			ZoneVersion:        "999",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		},
		))
	})

	It("should run and apply done status", func() {
		context := NewAsyncJobContext(helmClient, kubeClient, context.TODO(), syncManager)

		undeployJob.Run(context)

		result := &v1.AnyApplication{}
		_ = kubeClient.Get(context.GetGoContext(), client.ObjectKeyFromObject(application), result)

		Expect(result.Status.Conditions).To(Equal(
			[]v1.ConditionStatus{
				{
					Type:               v1.UndeploymenConditionType,
					ZoneId:             "zone",
					ZoneVersion:        "1000",
					Status:             string(v1.UndeploymentStatusDone),
					LastTransitionTime: fakeClock.NowTime(),
				},
			},
		))

		Expect(undeployJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.UndeploymenConditionType,
				ZoneId:             "zone",
				ZoneVersion:        "999",
				Status:             string(v1.UndeploymentStatusDone),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

	})

})
