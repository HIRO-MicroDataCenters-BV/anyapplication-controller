package global

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Local Application FSM", func() {
	var (
		fakeClock         clock.Clock
		runtimeConfig     config.ApplicationRuntimeConfig
		jobFactory        job.AsyncJobFactoryImpl
		application       v1.AnyApplication
		localApplication  mo.Option[local.LocalApplication]
		globalApplication types.GlobalApplication
		fakeEvents        events.Events
	)

	BeforeEach(func() {
		fakeClock = clock.NewFakeClock()
		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		fakeEvents = events.NewFakeEvents()
		jobFactory = job.NewAsyncJobFactory(&runtimeConfig, fakeClock, logf.Log, &fakeEvents)
		localApplication = mo.None[local.LocalApplication]()

		application = v1.AnyApplication{
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
				Owner: "otherzone",
				State: v1.UnknownGlobalState,
			},
		}
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)

	})

	It("should not react on new application", func() {
		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(v1.AnyApplicationStatus{}))

		Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should create start relocation once placement is done", func() {
		application.Status.Placements = []v1.Placement{{Zone: "zone"}}

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.RelocationConditionType,
						ZoneId:             "zone",
						Status:             string(v1.RelocationStatusPull),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		))

		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.RelocationConditionType,
			ZoneId:             "zone",
			Status:             string(v1.RelocationStatusPull),
			LastTransitionTime: fakeClock.NowTime(),
		}))

		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should create operational job once relocation is done", func() {
		application.Status.Placements = []v1.Placement{{Zone: "zone"}}
		application.Status.Conditions = []v1.ConditionStatus{
			{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusDone),
				LastTransitionTime: fakeClock.NowTime(),
			},
		}
		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)
		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.RelocationConditionType,
						ZoneId:             "zone",
						Status:             string(v1.RelocationStatusDone),
						LastTransitionTime: fakeClock.NowTime(),
					},
					{
						Type:               v1.LocalConditionType,
						ZoneId:             "zone",
						Status:             string(health.HealthStatusProgressing),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		))

		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: fakeClock.NowTime(),
		}))

		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should undeploy the application when placement has changed", func() {
		application.Status.Placements = []v1.Placement{{Zone: "otherzone"}}
		application.Status.Conditions = []v1.ConditionStatus{
			{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			},
		}
		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)
		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "otherzone"}},
				Owner:      "otherzone",
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.RelocationConditionType,
						ZoneId:             "zone",
						Status:             string(v1.RelocationStatusUndeploy),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		))

		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.RelocationConditionType,
			ZoneId:             "zone",
			Status:             string(v1.RelocationStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		}))

		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

	})

})
