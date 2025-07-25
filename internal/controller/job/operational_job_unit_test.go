package job

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var _ = Describe("LocalOperationJobUnitTests", func() {
	var (
		localJob       *LocalOperationJob
		application    *v1.AnyApplication
		scheme         *runtime.Scheme
		gitOpsEngine   *fixture.FakeGitOpsEngine
		kubeClient     client.Client
		fakeClock      *clock.FakeClock
		jobContext     types.AsyncJobContext
		runtimeConfig  config.ApplicationRuntimeConfig
		updateFuncs    []cache.UpdateSettingsFunc
		fakeHelmClient *helm.FakeHelmClient
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)

		fakeClock = clock.NewFakeClock()
		application = &v1.AnyApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: v1.AnyApplicationSpec{
				Source: v1.ApplicationSourceSpec{
					HelmSelector: &v1.ApplicationSourceHelm{
						Repository: "https://helm.nginx.com/stable",
						Chart:      "nginx-ingress",
						Version:    "2.0.1",
						Namespace:  "default",
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

		pod := corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					sync.LABEL_INSTANCE_ID:   "default-test-app",
					sync.LABEL_CHART_VERSION: "2.0.0",
					sync.LABEL_MANAGED_BY:    "dcp",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
				UID:               "test-pod-uid",
				ResourceVersion:   "1",
			},
			Spec: corev1.PodSpec{
				NodeName: "test-node",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "main",
						RestartCount: 1,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
						},
					},
				},
			},
		}

		gitOpsEngine = fixture.NewFakeGitopsEngine()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()

		fakeHelmClient = helm.NewFakeHelmClient()

		yamlData, err := yaml.Marshal(pod)
		if err != nil {
			panic(err)
		}
		fakeHelmClient.MockTemplate(string(yamlData))

		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollOperationalStatusInterval: 100 * time.Millisecond,
			PollSyncStatusInterval:        300 * time.Millisecond,
			DefaultSyncTimeout:            300 * time.Millisecond,
		}

		updateFuncs = []cache.UpdateSettingsFunc{
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				info = &types.ResourceInfo{ManagedByMark: un.GetLabels()[sync.LABEL_MANAGED_BY]}
				cacheManifest = true
				return
			}),
		}

		clusterCache, _ := fixture.NewTestClusterCacheWithOptions(updateFuncs)
		charts := sync.NewCharts(context.TODO(), helmClient, &sync.ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)
		applications := sync.NewApplications(kubeClient, fakeHelmClient, charts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		jobContext = NewAsyncJobContext(fakeHelmClient, kubeClient, ctx, applications)

		localJob = NewLocalOperationJob(application, &runtimeConfig, fakeClock, logf.Log, &fakeEvents)
	})

	It("LocalOperationJob should exit with failure if resources are missing", func() {
		zoneStatus := application.Status.GetOrCreateStatusFor("zone")
		zoneStatus.ChartVersion = "2.0.1"

		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		go localJob.Run(jobContext)

		fakeClock.Advance(1 * time.Second)

		waitForJobStatus(localJob, string(health.HealthStatusMissing))

		status := localJob.GetStatus()
		Expect(status.Status).To(Equal(string(health.HealthStatusMissing)))
		Expect(status.Msg).To(Equal("Operation Failure: Application resources are missing"))

	})

	It("LocalOperationJob should exit with failure if new version is available", func() {

		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		go localJob.Run(jobContext)

		fakeClock.Advance(1 * time.Second)

		waitForJobStatus(localJob, string(health.HealthStatusProgressing))

		status := localJob.GetStatus()
		Expect(status.Status).To(Equal(string(health.HealthStatusProgressing)))
		Expect(status.Msg).To(Equal("Operation Failure: Newer version '2.0.1' is available"))

	})

})
