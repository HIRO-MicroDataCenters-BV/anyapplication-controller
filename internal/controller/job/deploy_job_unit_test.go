package job

import (
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/fixture"

	ctrl_sync "hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DeployJobUnitests", func() {
	var (
		deployJob     *DeployJob
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		gitOpsEngine  *fixture.FakeGitOpsEngine
		kubeClient    client.Client
		fakeClock     *clock.FakeClock
		jobContext    types.AsyncJobContext
		runtimeConfig config.ApplicationRuntimeConfig
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
						Namespace:  "test",
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
		gitOpsEngine = fixture.NewFakeGitopsEngine()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		helmClient = helm.NewFakeHelmClient()
		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollOperationalStatusInterval: 100 * time.Millisecond,
			PollSyncStatusInterval:        300 * time.Millisecond,
			DefaultSyncTimeout:            300 * time.Millisecond,
		}

		clusterCache, _ := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})
		fakeCharts := ctrl_sync.NewFakeCharts()
		applications := ctrl_sync.NewApplications(kubeClient, helmClient, fakeCharts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		jobContext = NewAsyncJobContext(helmClient, kubeClient, ctx, applications)

		deployJob = NewDeployJob(application, &runtimeConfig, fakeClock, logf.Log, &fakeEvents)
	})

	It("Deployment should retry and fail after several attempts", func() {
		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		go deployJob.Run(jobContext)

		fakeClock.Advance(1 * time.Second)

		status := deployJob.GetStatus()
		Expect(status.Status).To(Equal(string(v1.DeploymentStatusPull)))
		waitForJobMsg(deployJob, "Deployment failure: Retrying deployment (attempt 2 of 3)")

		fakeClock.Advance(1 * time.Second)

		status = deployJob.GetStatus()
		Expect(status.Status).To(Equal(string(v1.DeploymentStatusPull)))
		waitForJobMsg(deployJob, "Deployment failure: Retrying deployment (attempt 3 of 3)")

		fakeClock.Advance(1 * time.Second)

		waitForJobStatus(deployJob, string(v1.DeploymentStatusFailure))

		Expect(deployJob.GetStatus().Msg).To(Equal("Deployment failure: Deployment timed out after 300ms"))
	})

})
