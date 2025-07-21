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
				Name:            "test-app",
				Namespace:       "default",
				ResourceVersion: "1",
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

	It("should start deployment once placement is done", func() {
		application.Status.Placements = []v1.Placement{{Zone: "zone"}}

		Expect(globalApplication.IsDeployed()).To(BeFalse())
		Expect(globalApplication.IsPresent()).To(BeFalse())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 0,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.DeploymentConditionType,
								ZoneId:             "zone",
								Status:             string(v1.DeploymentStatusPull),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		))

		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.DeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.DeploymentStatusPull),
			LastTransitionTime: fakeClock.NowTime(),
		}))

		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should avoid double start deployment job if one is already running", func() {
		deploymentCondition := v1.ConditionStatus{
			Type:               v1.DeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.DeploymentStatusPull),
			LastTransitionTime: fakeClock.NowTime(),
		}

		application.Status.Placements = []v1.Placement{{Zone: "zone"}}
		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:      "zone",
				ZoneVersion: 1,
				Conditions:  []v1.ConditionStatus{deploymentCondition},
			},
		}

		Expect(globalApplication.IsDeployed()).To(BeFalse())
		Expect(globalApplication.IsPresent()).To(BeFalse())

		statusResult := globalApplication.DeriveNewStatus(
			types.FromCondition(deploymentCondition, types.AsyncJobTypeDeploy), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.DeploymentConditionType,
								ZoneId:             "zone",
								Status:             string(v1.DeploymentStatusPull),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		))

		Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should create operational job once deployement is done", func() {
		application.Status.Placements = []v1.Placement{{Zone: "zone"}}

		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:      "zone",
				ZoneVersion: 1,
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.DeploymentConditionType,
						ZoneId:             "zone",
						Status:             string(v1.DeploymentStatusDone),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		}

		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig, fakeClock, true))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.DeploymentConditionType,
								ZoneId:             "zone",
								Status:             string(v1.DeploymentStatusDone),
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

	It("should avoid double start operational job if it is in progress", func() {
		operationalCondition := v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: fakeClock.NowTime(),
		}

		application.Status.Placements = []v1.Placement{{Zone: "zone"}}

		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:      "zone",
				ZoneVersion: 1,
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.DeploymentConditionType,
						ZoneId:             "zone",
						Status:             string(v1.DeploymentStatusDone),
						LastTransitionTime: fakeClock.NowTime(),
					},
					operationalCondition,
				},
			},
		}

		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig, fakeClock, true))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.FromCondition(operationalCondition, types.AsyncJobTypeLocalOperation), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "zone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.LocalConditionType,
								ZoneId:             "zone",
								Status:             string(health.HealthStatusProgressing),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		))

		Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
	})

	It("should undeploy the application when placement has changed", func() {
		application.Status.Placements = []v1.Placement{{Zone: "otherzone"}}

		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:      "zone",
				ZoneVersion: 1,
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.LocalConditionType,
						ZoneId:             "zone",
						Status:             string(health.HealthStatusProgressing),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		}

		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig, fakeClock, true))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "otherzone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.UndeploymentConditionType,
								ZoneId:             "zone",
								Status:             string(v1.UndeploymentStatusUndeploy),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		))

		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.UndeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		}))

		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

	})

	It("should avoid double starting undeploy job if the undeploy job is already in progress", func() {
		undeployCondition := v1.ConditionStatus{
			Type:               v1.UndeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		}

		application.Status.Placements = []v1.Placement{{Zone: "otherzone"}}

		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:      "zone",
				ZoneVersion: 1,
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.LocalConditionType,
						ZoneId:             "zone",
						Status:             string(health.HealthStatusProgressing),
						LastTransitionTime: fakeClock.NowTime(),
					},
					undeployCondition,
				},
			},
		}

		localApplication = mo.Some(local.FakeLocalApplication(&runtimeConfig, fakeClock, true))
		globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.FromCondition(undeployCondition, types.AsyncJobTypeUndeploy), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				State:      v1.UnknownGlobalState,
				Placements: []v1.Placement{{Zone: "otherzone"}},
				Owner:      "otherzone",
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 1,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.UndeploymentConditionType,
								ZoneId:             "zone",
								Status:             string(v1.UndeploymentStatusUndeploy),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		))

		Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

	})

})
