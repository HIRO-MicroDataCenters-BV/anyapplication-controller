package global

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/local"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO job start/ stop
var _ = Describe("GlobalApplication", func() {
	clock := clock.NewFakeClock()
	Context("When new resource is created", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		localApplication := mo.None[local.LocalApplication]()

		It("transit to placement state", func() {
			applicationResource := makeApplication()
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)
			jobFactory := job.NewAsyncJobFactory(runtimeConfig, clock)

			statusResult := globalApplication.DeriveNewStatus(EmptyJobConditions(), jobFactory)
			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs
			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State:      v1.PlacementGlobalState,
					Placements: nil,
					Owner:      "zone",
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             "zone",
							Status:             string(v1.PlacementStatusInProgress),
							LastTransitionTime: clock.NowTime(),
						},
					},
				},
			))

			jobToAdd := jobs.jobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: clock.NowTime(),
			}))

			Expect(jobs.jobsToRemove).To(Equal(mo.None[job.AsyncJobType]()))
		})
	})

	Context("placement state", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		jobFactory := job.NewAsyncJobFactory(runtimeConfig, clock)

		It("create local placement job", func() {
			applicationResource := makeApplication()
			applicationResource.Status.Owner = "zone"
			applicationResource.Status.State = v1.PlacementGlobalState

			localApplication := mo.None[local.LocalApplication]()
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)
			existingJobCondition := FromCondition(&v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: clock.NowTime(),
			})
			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.PlacementGlobalState,
					Owner: "zone",
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             "zone",
							Status:             string(v1.PlacementStatusInProgress),
							LastTransitionTime: clock.NowTime(),
						},
					},
				},
			))

			Expect(jobs.jobsToAdd).To(Equal(mo.None[job.AsyncJob]()))
			Expect(jobs.jobsToRemove).To(Equal(mo.None[job.AsyncJobType]()))
		})

		It("switch to operational once placement is done", func() {
			applicationResource := makeApplication()
			applicationResource.Status.Owner = "zone"
			applicationResource.Status.State = v1.PlacementGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "zone"}}
			applicationResource.Status.Conditions = []v1.ConditionStatus{
				{
					Type:               v1.PlacementConditionType,
					ZoneId:             "zone",
					Status:             string(v1.PlacementStatusDone),
					LastTransitionTime: clock.NowTime(),
				},
			}
			existingJobCondition := FromCondition(&v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusDone),
				LastTransitionTime: clock.NowTime(),
			})

			localApplication := mo.None[local.LocalApplication]()
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: "zone"},
					},
					Owner: "zone",
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             "zone",
							Status:             string(v1.PlacementStatusDone),
							LastTransitionTime: clock.NowTime(),
						},
						{
							Type:               v1.RelocationConditionType,
							ZoneId:             "zone",
							Status:             string(v1.RelocationStatusPull),
							LastTransitionTime: clock.NowTime(),
						},
						// {
						// 	Type:               v1.LocalConditionType,
						// 	ZoneId:             "zone",
						// 	Status:             string(health.HealthStatusHealthy),
						// 	LastTransitionTime: clock.NowTime(),
						// },
					},
				},
			))

			jobToAdd := jobs.jobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusPull),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.jobsToRemove).To(Equal(mo.None[job.AsyncJobType]()))
		})

		It("switch to operational once placement is done", func() {
			applicationResource := makeApplication()
			applicationResource.Status.Owner = "zone"
			applicationResource.Status.State = v1.PlacementGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "zone"}}
			applicationResource.Status.Conditions = []v1.ConditionStatus{
				{
					Type:               v1.PlacementConditionType,
					ZoneId:             "zone",
					Status:             string(v1.PlacementStatusDone),
					LastTransitionTime: clock.NowTime(),
				},
			}
			existingJobCondition := FromCondition(&v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusDone),
				LastTransitionTime: clock.NowTime(),
			})

			localApplication := mo.None[local.LocalApplication]()
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: "zone"},
					},
					Owner: "zone",
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             "zone",
							Status:             string(v1.PlacementStatusDone),
							LastTransitionTime: clock.NowTime(),
						},
						{
							Type:               v1.RelocationConditionType,
							ZoneId:             "zone",
							Status:             string(v1.RelocationStatusPull),
							LastTransitionTime: clock.NowTime(),
						},
						// {
						// 	Type:               v1.LocalConditionType,
						// 	ZoneId:             "zone",
						// 	Status:             string(health.HealthStatusHealthy),
						// 	LastTransitionTime: clock.NowTime(),
						// },
					},
				},
			))

			jobToAdd := jobs.jobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusPull),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.jobsToRemove).To(Equal(mo.None[job.AsyncJobType]()))
		})

	})
})

func makeApplication() *v1.AnyApplication {
	application := &v1.AnyApplication{
		ObjectMeta: metav1.ObjectMeta{},
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
	}

	return application

}
