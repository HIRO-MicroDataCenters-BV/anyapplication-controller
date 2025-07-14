package job

import (
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/sync"
	types "hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/textlogger"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("LocalOperationJob", func() {
	var (
		operationJob  *LocalOperationJob
		helmClient    helm.HelmClient
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		theClock      clock.Clock
		runtimeConfig config.ApplicationRuntimeConfig
		jobContext    types.AsyncJobContext
		fakeEvents    events.Events
		stopFunc      engine.StopFunc
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

		pollSyncStatusInterval, _ := time.ParseDuration("1000ms")
		pollOperationalStatusInterval, _ := time.ParseDuration("1000ms")
		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollSyncStatusInterval:        pollSyncStatusInterval,
			PollOperationalStatusInterval: pollOperationalStatusInterval,
		}
		var err error
		helmClient, err = helm.NewHelmClient(&helm.HelmClientOptions{
			RestConfig: cfg,
			Debug:      false,
			Linting:    true,
			KubeVersion: &chartutil.KubeVersion{
				Version: "v1.23.10",
				Major:   "1",
				Minor:   "23",
			},
			UpgradeCRDs: true,
		})
		if err != nil {
			panic("error " + err.Error())
		}

		theClock = clock.NewClock()
		fakeEvents = events.NewFakeEvents()
		log := textlogger.NewLogger(textlogger.NewConfig())
		clusterCache := cache.NewClusterCache(cfg,
			cache.SetLogr(log),
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				managedByMark := un.GetLabels()["dcp.hiro.io/managed-by"]
				info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
				// cache resources that has that mark to improve performance
				cacheManifest = managedByMark != ""
				return
			}),
		)
		gitOpsEngine := engine.NewEngine(cfg, clusterCache, engine.WithLogr(log))
		stopFunc, err = gitOpsEngine.Run()
		if err != nil {
			panic("error " + err.Error())
		}

		syncManager := sync.NewSyncManager(k8sClient, helmClient, clusterCache, theClock, &runtimeConfig, gitOpsEngine, logf.Log)
		jobContext = NewAsyncJobContext(helmClient, k8sClient, ctx, syncManager)
		operationJob = NewLocalOperationJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)

	})

	AfterEach(func() {
		stopFunc()
	})

	It("should return initial status", func() {
		status := operationJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: metav1.Time{},
		},
		))
	})

	It("should sync periodically and report status", func() {

		deployJob := NewDeployJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)
		deployJob.Run(jobContext)
		waitForDeploymentStatus(deployJob, string(v1.DeploymentStatusDone))
		deployJob.Stop()

		operationJob.Run(jobContext)

		waitForStatus(operationJob, health.HealthStatusProgressing)

		status := operationJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: metav1.Time{},
				Msg:                "\n",
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
