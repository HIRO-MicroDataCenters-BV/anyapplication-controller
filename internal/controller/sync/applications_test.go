package sync

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
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Applications", func() {
	var (
		fakeClock          clock.Clock
		applications       types.Applications
		kubeClient         client.Client
		helmClient         helm.HelmClient
		application        *v1.AnyApplication
		scheme             *runtime.Scheme
		clusterCache       cache.ClusterCache
		clusterCacheClient *k8sfake.FakeDynamicClient
		runtimeConfig      config.ApplicationRuntimeConfig
		gitOpsEngine       *fixture.FakeGitOpsEngine
		updateFuncs        []cache.UpdateSettingsFunc
		charts             types.Charts
	)

	BeforeEach(func() {
		fakeClock = clock.NewFakeClock()
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollOperationalStatusInterval: 100 * time.Millisecond,
		}

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
						Version:    "~2.0.0",
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

		application = application.DeepCopy()
	})

	BeforeEach(func() {
		config := &rest.Config{
			Host: "https://test",
		}
		options := helm.HelmClientOptions{
			RestConfig: config,
			KubeVersion: &chartutil.KubeVersion{
				Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
				Major:   "1",
				Minor:   "23",
			},
		}
		helmClient, _ = helm.NewHelmClient(&options)

		clusterCache, clusterCacheClient = fixture.NewTestClusterCacheWithOptions(updateFuncs)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&v1.AnyApplication{}).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					gvk := obj.GetObjectKind().GroupVersionKind()
					resourcePlural, _ := meta.UnsafeGuessKindToResource(gvk)
					err := clusterCacheClient.Tracker().Delete(
						gvk.GroupVersion().WithResource(resourcePlural.Resource),
						obj.GetNamespace(),
						obj.GetName(),
					)
					return err
				},
			}).
			Build()

		updateFuncs = []cache.UpdateSettingsFunc{
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				info = &types.ResourceInfo{ManagedByMark: un.GetLabels()[LABEL_MANAGED_BY]}
				cacheManifest = true
				return
			}),
		}

		gitOpsEngine = fixture.NewFakeGitopsEngine()
		charts = NewCharts(context.TODO(), helmClient, &ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)
		applications = NewApplications(kubeClient, helmClient, charts,
			clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)
	})

	It("should get target version for the application", func() {
		versionOpt := applications.GetTargetVersion(application)
		_, ok := versionOpt.Get()
		Expect(ok).To(BeFalse())

		// Set the active version in the application status
		zoneStatus := application.Status.GetOrCreateStatusFor("zone")
		zoneStatus.ChartVersion = "2.0.1"

		versionOpt = applications.GetTargetVersion(application)
		version, ok := versionOpt.Get()
		Expect(ok).To(BeTrue())
		Expect(version.ToString()).To(Equal("2.0.1"))
	})

	It("should get aggregated status for the version of application", func() {
		pod200 := makePod("test-pod1", "2.0.0")
		pod201 := makePod("test-pod2", "2.0.1")

		clusterCache, _ = fixture.NewTestClusterCacheWithOptions(updateFuncs, &pod200, &pod201)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		charts = NewCharts(context.TODO(), helmClient, &ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)
		applications = NewApplications(kubeClient, helmClient, charts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		version201, _ := types.NewSpecificVersion("2.0.1")
		_, err := applications.SyncVersion(context.Background(), application, version201)
		Expect(err).NotTo(HaveOccurred())

		version200, _ := types.NewSpecificVersion("2.0.0")
		_, err = applications.SyncVersion(context.Background(), application, version200)
		Expect(err).NotTo(HaveOccurred())

		versions, _ := applications.GetAllPresentVersions(application)
		Expect(versions.ToSlice()).To(HaveLen(2))

		status := applications.GetAggregatedStatusVersion(application, version200)
		Expect(status.HealthStatus.Status).To(Equal(health.HealthStatusMissing))
		Expect(status.ChartVersion.ToString()).To(Equal("2.0.0"))

		status = applications.GetAggregatedStatusVersion(application, version201)
		Expect(status.HealthStatus.Status).To(Equal(health.HealthStatusMissing))
		Expect(status.ChartVersion.ToString()).To(Equal("2.0.1"))

	})

	It("should cleanup all version", func() {
		pod200 := makePod("test-pod1", "2.0.0")
		pod201 := makePod("test-pod2", "2.0.1")

		clusterCache, clusterCacheClient := fixture.NewTestClusterCacheWithOptions(updateFuncs, &pod200, &pod201)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&v1.AnyApplication{}).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					gvk := obj.GetObjectKind().GroupVersionKind()
					resourcePlural, _ := meta.UnsafeGuessKindToResource(gvk)
					err := clusterCacheClient.Tracker().Delete(
						gvk.GroupVersion().WithResource(resourcePlural.Resource),
						obj.GetNamespace(),
						obj.GetName(),
					)
					return err
				},
			}).
			WithObjects(application, &pod200, &pod201).
			Build()

		charts = NewCharts(context.Background(), helmClient, &ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)
		applications = NewApplications(kubeClient, helmClient, charts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		result, err := applications.Cleanup(context.Background(), application)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(2))

		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		versions, _ := applications.GetAllPresentVersions(application)
		Expect(versions.ToSlice()).To(BeEmpty())
	})

	It("should determine target version for the application if version is not set for zone", func() {
		targetVersion, _ := applications.DetermineTargetVersion(application)
		Expect(targetVersion.ToString()).To(Equal("2.0.1"))
	})

	It("should determine target version for the application if latest version is set for zone", func() {
		zoneStatus := application.Status.GetOrCreateStatusFor("zone")
		zoneStatus.ChartVersion = "2.0.1"

		targetVersion, _ := applications.DetermineTargetVersion(application)
		Expect(targetVersion.ToString()).To(Equal("2.0.1"))
	})

	It("should determine target version for the application if old version is set for zone", func() {
		zoneStatus := application.Status.GetOrCreateStatusFor("zone")
		zoneStatus.ChartVersion = "2.0.0"

		targetVersion, _ := applications.DetermineTargetVersion(application)
		Expect(targetVersion.ToString()).To(Equal("2.0.1"))
	})

	It("should return unique instance id for helm chart version and release", func() {
		instanceId := applications.GetInstanceId(application)

		Expect(instanceId).To(Equal("default-test-app"))
	})

	It("should return aggregated status for application when application is missing", func() {
		version, _ := types.NewSpecificVersion("2.0.1")
		status := applications.GetAggregatedStatusVersion(application, version)

		Expect(status.HealthStatus.Status).To(Equal(health.HealthStatusMissing))
		Expect(status.HealthStatus.Message).To(Equal(". "))
	})

	It("should load application from cluster cache", func() {
		globalApplication, _ := applications.LoadApplication(application)

		Expect(globalApplication.IsDeployed()).To(BeFalse())
		Expect(globalApplication.IsPresent()).To(BeFalse())
		Expect(globalApplication.IsVersionChanged()).To(BeTrue())

		versions, _ := applications.GetAllPresentVersions(application)
		Expect(versions.ToSlice()).To(BeEmpty())
	})

	It("should sync helm release", func() {

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

		newVersion, _ := types.NewSpecificVersion("2.0.1")

		syncResult, err := applications.SyncVersion(context.Background(), application, newVersion)

		Expect(err).NotTo(HaveOccurred())
		Expect(syncResult.Total).To(Equal(2))
		Expect(syncResult.OperationPhaseStats).To(Equal(map[common.OperationPhase]int{
			"Succeeded": 2,
		}))
		Expect(syncResult.SyncPhaseStats).To(Equal(map[common.SyncPhase]int{
			"Sync": 2,
		}))
		Expect(syncResult.ResultCodeStats).To(Equal(map[common.ResultCode]int{
			"Synced": 2,
		}))

		Expect(syncResult.AggregatedStatus.HealthStatus.Status).To(Equal(health.HealthStatusMissing))
	})

	It("should delete helm release or fail", func() {
		version, _ := types.NewSpecificVersion("2.0.1")
		_, err := applications.SyncVersion(context.Background(), application, version)
		Expect(err).NotTo(HaveOccurred())

		syncResult, err := applications.DeleteVersion(context.TODO(), application, version)

		Expect(err).NotTo(HaveOccurred())
		Expect(syncResult.Total).To(Equal(23))
		Expect(syncResult.Deleted).To(Equal(23))
		Expect(syncResult.DeleteFailed).To(Equal(0))
	})

})

func makePod(name string, version string) corev1.Pod {
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				LABEL_INSTANCE_ID:   "default-test-app",
				LABEL_CHART_VERSION: version,
				LABEL_MANAGED_BY:    LABEL_VALUE_MANAGED_BY_DCP,
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
	return pod
}
