package job

import (
	"context"
	"time"

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

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs", func() {
	var (
		ctx           context.Context
		kubeClient    client.Client
		helmClient    helm.HelmClient
		application   *v1.AnyApplication
		scheme        *runtime.Scheme
		fakeClock     clock.Clock
		jobs          types.AsyncJobs
		runtimeConfig config.ApplicationRuntimeConfig
		gitOpsEngine  *fixture.FakeGitOpsEngine
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeClock = clock.NewFakeClock()
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)

		application = &v1.AnyApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: v1.AnyApplicationSpec{
				Source: v1.ApplicationSourceSpec{
					HelmSelector: &v1.ApplicationSourceHelm{
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
				Ownership: v1.OwnershipStatus{
					Epoch: 1,
					Owner: "zone",
					State: v1.PlacementGlobalState,
				},
				Zones: []v1.ZoneStatus{
					{
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.PlacementConditionType,
								ZoneId:             "zone",
								Status:             string(v1.PlacementStatusInProgress),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		}
		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollOperationalStatusInterval: 100 * time.Millisecond,
		}
		gitOpsEngine = fixture.NewFakeGitopsEngine()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		helmClient = helm.NewFakeHelmClient()

		clusterCache, _ := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})
		fakeCharts := ctrl_sync.NewFakeCharts()
		applications := ctrl_sync.NewApplications(kubeClient, helmClient, fakeCharts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		context := NewAsyncJobContext(helmClient, kubeClient, ctx, applications)
		jobs = NewJobs(context)
	})

	It("should run job and get completion status", func() {
		appId := types.ApplicationId{
			Name:      "test",
			Namespace: "test",
		}
		job := newTestJob(appId, types.AsyncJobTypeLocalOperation, fakeClock, 100*time.Millisecond)

		jobs.Execute(job)
		currentJobOpt := jobs.GetCurrent(appId)

		Expect(currentJobOpt.IsPresent()).To(BeTrue())
		currentJob := currentJobOpt.OrEmpty()

		Expect(currentJob.GetStatus()).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			},
		))

		jobs.Stop(appId)
		currentJobOpt = jobs.GetCurrent(appId)
		Expect(currentJobOpt.IsPresent()).To(BeFalse())
	})

})

type testJob struct {
	id       types.JobId
	clock    clock.Clock
	interval time.Duration
	status   health.HealthStatusCode
}

func newTestJob(applicationId types.ApplicationId, jobType types.AsyncJobType, clock clock.Clock, interval time.Duration) types.AsyncJob {
	return &testJob{
		clock: clock,
		id: types.JobId{
			JobType:       jobType,
			ApplicationId: applicationId,
		},
		status:   health.HealthStatusProgressing,
		interval: interval,
	}
}

func (j *testJob) GetJobID() types.JobId {
	return j.id
}
func (j *testJob) GetType() types.AsyncJobType {
	return j.id.JobType
}
func (j *testJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.LocalConditionType,
		ZoneId:             "zone",
		Status:             string(j.status),
		LastTransitionTime: j.clock.NowTime(),
	}
}
func (j *testJob) Run(jobContext types.AsyncJobContext) {

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.runInner()
		case <-jobContext.GetGoContext().Done():
			return
		}
	}
}

func (j *testJob) runInner() {
	j.status = health.HealthStatusProgressing
}
