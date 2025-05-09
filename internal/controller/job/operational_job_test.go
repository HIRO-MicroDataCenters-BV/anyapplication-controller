package job

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("LocalOperationJob", func() {
	var (
		operationJob  *LocalOperationJob
		kubeClient    client.Client
		helmClient    helm.HelmClient
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		fakeClock     *clock.FakeClock
		runtimeConfig config.ApplicationRuntimeConfig
		jobContext    types.AsyncJobContext
		gitOpsEngine  *fixture.FakeGitOpsEngine
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)

		application = &v1.AnyApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: v1.AnyApplicationSpec{
				Application: v1.ApplicationMatcherSpec{
					HelmSelector: &v1.HelmSelectorSpec{
						Repository: "https://helm.nginx.com/stable",
						Chart:      "nginx-ingress",
						Version:    "2.0.1",
						Namespace:  "nginx",
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
			ZoneId:            "zone",
			LocalPollInterval: 10000 * time.Millisecond,
		}
		gitOpsEngine = fixture.NewFakeGitopsEngine()
		fakeClock = clock.NewFakeClock()

		options := helm.HelmClientOptions{
			RestConfig: &rest.Config{Host: "https://test"},
			KubeVersion: &chartutil.KubeVersion{
				Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
				Major:   "1",
				Minor:   "23",
			},
		}
		helmClient, _ = helm.NewHelmClient(&options)

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		application = application.DeepCopy()

		clusterCache := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})
		syncManager := sync.NewSyncManager(kubeClient, helmClient, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine)
		jobContext = NewAsyncJobContext(helmClient, kubeClient, context.TODO(), syncManager)

		operationJob = NewLocalOperationJob(application, &runtimeConfig, fakeClock)
	})

	It("should return initial status", func() {
		Expect(operationJob.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: fakeClock.NowTime(),
		},
		))
	})

	It("should sync periodically and report status", func() {

		gitOpsEngine.MockSyncResult([]common.ResourceSyncResult{
			{
				ResourceKey: kube.NewResourceKey("group", "kind", "namespace", "test-app1"),
				Version:     "1.0.0",
				Order:       1,
				Status:      common.ResultCodeSynced,
				Message:     "message",
				HookType:    common.HookTypeSync,
				HookPhase:   common.OperationSucceeded,
				SyncPhase:   common.SyncPhaseSync,
			},
			{
				ResourceKey: kube.NewResourceKey("group", "kind", "namespace", "test-app2"),
				Version:     "1.0.0",
				Order:       2,
				Status:      common.ResultCodeSynced,
				Message:     "message",
				HookType:    common.HookTypeSync,
				HookPhase:   common.OperationSucceeded,
				SyncPhase:   common.SyncPhaseSync,
			},
		})

		Expect(operationJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

		operationJob.Run(jobContext)

		waitForStatus(operationJob, health.HealthStatusUnknown)

		Expect(operationJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusUnknown),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

		operationJob.Stop()

	})

	It("should sync periodically and report failure", func() {
		application.Spec.Application.HelmSelector = &v1.HelmSelectorSpec{
			Repository: "test-repo",
			Chart:      "test-chart",
			Version:    "1.0.0",
		}
		operationJob = NewLocalOperationJob(application, &runtimeConfig, fakeClock)

		Expect(operationJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

		operationJob.Run(jobContext)

		waitForStatus(operationJob, health.HealthStatusDegraded)

		Expect(operationJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusDegraded),
				LastTransitionTime: fakeClock.NowTime(),
				Msg:                "Fail to render application: Helm template failure: Failed to AddOrUpdateChartRepo: could not find protocol handler for: ",
			},
		))

		operationJob.Stop()

	})

})

func waitForStatus(job *LocalOperationJob, status health.HealthStatusCode) {
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if job.GetStatus().Status == string(status) {
			return
		}
	}
	Fail(fmt.Sprintf("Expected status %s, but got %s", status, job.GetStatus().Status))
}
