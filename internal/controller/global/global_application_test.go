package global

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GlobalApplication", func() {
	clock := clock.NewFakeClock()

	Context("When new resource is created", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		localApplication := mo.None[local.LocalApplication]()

		It("transit to placement state", func() {
			applicationResource := makeApplication()
			applicationResource.Status.Owner = ""
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)
			jobFactory := job.NewAsyncJobFactory(runtimeConfig, clock)

			statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)
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

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: clock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})
	})

	Context("placement state", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		jobFactory := job.NewAsyncJobFactory(runtimeConfig, clock)

		It("create local placement job", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.PlacementGlobalState

			localApplication := mo.None[local.LocalApplication]()
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)
			existingJobCondition := types.FromCondition(v1.ConditionStatus{
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

			Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to relocation once placement is done", func() {
			applicationResource := makeApplication()
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
			existingJobCondition := types.FromCondition(v1.ConditionStatus{
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
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusPull),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch finish operational once relocation is completed ", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.RelocationGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "zone"}}
			applicationResource.Status.Conditions = []v1.ConditionStatus{
				{
					Type:               v1.PlacementConditionType,
					ZoneId:             "zone",
					Status:             string(v1.PlacementStatusDone),
					LastTransitionTime: clock.NowTime(),
				},
				{
					Type:               v1.RelocationConditionType,
					ZoneId:             "zone",
					Status:             string(v1.RelocationStatusDone),
					LastTransitionTime: clock.NowTime(),
				},
			}
			existingJobCondition := types.FromCondition(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusDone),
				LastTransitionTime: clock.NowTime(),
			})

			localApplication := mo.Some(local.FakeLocalApplication(runtimeConfig))
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.OperationalGlobalState,
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
							Status:             string(v1.RelocationStatusDone),
							LastTransitionTime: clock.NowTime(),
						},
						{
							Type:               v1.LocalConditionType,
							ZoneId:             "zone",
							Status:             string(health.HealthStatusProgressing),
							LastTransitionTime: clock.NowTime(),
						},
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("should relocate if placements has changed", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "zone"}}
			applicationResource.Status.Conditions = []v1.ConditionStatus{
				{
					Type:               v1.LocalConditionType,
					ZoneId:             "otherzone",
					Status:             string(health.HealthStatusProgressing),
					LastTransitionTime: clock.NowTime(),
				},
			}
			existingJobCondition := types.EmptyJobConditions()

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
							Type:               v1.LocalConditionType,
							ZoneId:             "otherzone",
							Status:             string(health.HealthStatusProgressing),
							LastTransitionTime: clock.NowTime(),
						},
						{
							Type:               v1.RelocationConditionType,
							ZoneId:             "zone",
							Status:             string(v1.RelocationStatusPull),
							LastTransitionTime: clock.NowTime(),
						},
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusPull),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("should undeploy if current zone not in placements anymore", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "otherzone"}}
			applicationResource.Status.Conditions = []v1.ConditionStatus{
				{
					Type:               v1.LocalConditionType,
					ZoneId:             "zone",
					Status:             string(health.HealthStatusProgressing),
					LastTransitionTime: clock.NowTime(),
				},
				{
					Type:               v1.LocalConditionType,
					ZoneId:             "otherzone",
					Status:             string(health.HealthStatusProgressing),
					LastTransitionTime: clock.NowTime(),
				},
			}
			existingJobCondition := types.EmptyJobConditions()

			localApplication := mo.Some(local.FakeLocalApplication(runtimeConfig))
			globalApplication := NewFromLocalApplication(localApplication, clock, applicationResource, runtimeConfig)

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.OperationalGlobalState,
					Placements: []v1.Placement{
						{Zone: "otherzone"},
					},
					Owner: "zone",
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.LocalConditionType,
							ZoneId:             "otherzone",
							Status:             string(health.HealthStatusProgressing),
							LastTransitionTime: clock.NowTime(),
						},
						{
							Type:               v1.RelocationConditionType,
							ZoneId:             "zone",
							Status:             string(v1.RelocationStatusUndeploy),
							LastTransitionTime: clock.NowTime(),
						},
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.RelocationConditionType,
				ZoneId:             "zone",
				Status:             string(v1.RelocationStatusUndeploy),
				LastTransitionTime: clock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
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
		Status: v1.AnyApplicationStatus{
			Owner: "zone",
		},
	}

	return application

}
