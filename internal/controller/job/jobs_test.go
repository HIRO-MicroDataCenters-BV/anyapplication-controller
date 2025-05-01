package job

import (
	"context"
	"sync"
	"time"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/controller/fixture"
	ctrl_sync "hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs", func() {
	var (
		ctx         context.Context
		kubeClient  client.Client
		helmClient  helm.HelmClient
		application *v1.AnyApplication
		scheme      *runtime.Scheme
		fakeClock   clock.Clock
		jobs        types.AsyncJobs
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
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.PlacementConditionType,
						ZoneId:             "zone",
						Status:             string(v1.PlacementStatusInProgress),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		}

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		helmClient = helm.NewFakeHelmClient()

		clusterCache := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})
		syncManager := ctrl_sync.NewSyncManager(kubeClient, helmClient, clusterCache)

		context := NewAsyncJobContext(helmClient, kubeClient, ctx, syncManager)
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
	stopCh   chan struct{}
	wg       sync.WaitGroup
	status   health.HealthStatusCode
}

func newTestJob(applicationId types.ApplicationId, jobType types.AsyncJobType, clock clock.Clock, interval time.Duration) types.AsyncJob {
	return &testJob{
		clock: clock,
		id: types.JobId{
			JobType:       jobType,
			ApplicationId: applicationId,
		},
		stopCh:   make(chan struct{}),
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
func (j *testJob) Run(context types.AsyncJobContext) {
	j.wg.Add(1)

	go func() {
		defer j.wg.Done()

		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				j.runInner()
			case <-j.stopCh:
				return
			}
		}
	}()

}

func (j *testJob) runInner() {
	j.status = health.HealthStatusProgressing
}

func (job *testJob) Stop() {
	close(job.stopCh)
	job.wg.Wait()
}
