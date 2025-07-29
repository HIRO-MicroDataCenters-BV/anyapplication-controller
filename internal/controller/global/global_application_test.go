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

const CURRENT_ZONE = "zone"

var _ = Describe("GlobalApplication", func() {
	fakeClock := clock.NewFakeClock()

	Context("on application resource change", func() {
		currentZone := CURRENT_ZONE
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: currentZone,
		}
		events := events.NewFakeEvents()
		jobFactory := job.NewAsyncJobFactory(runtimeConfig, fakeClock, logf.Log, &events)
		version, _ := types.NewSpecificVersion("1.0.0")
		newVersion, _ := types.NewSpecificVersion("1.1.0")

		It("transit to placement state", func() {
			applicationResource := makeApplication()
			applicationResource.Status.Owner = ""
			localApplication := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplication,
				mo.Some(version), mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log,
			)
			jobFactory := job.NewAsyncJobFactory(runtimeConfig, fakeClock, logf.Log, &events)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)
			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs
			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State:      v1.PlacementGlobalState,
					Placements: nil,
					Owner:      currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 0,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.PlacementConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.PlacementStatusInProgress),
									LastTransitionTime: fakeClock.NowTime(),
								},
							},
						},
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             currentZone,
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("create local placement job", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.PlacementGlobalState

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version), mo.None[*types.SpecificVersion](),
				fakeClock, applicationResource, runtimeConfig, logf.Log)
			existingJobCondition := types.EmptyJobConditions()

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.PlacementGlobalState,
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 0,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.PlacementConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.PlacementStatusInProgress),
									LastTransitionTime: fakeClock.NowTime(),
								},
							},
						},
					},
				},
			))

			jobToAdd := jobs.JobsToAdd.OrEmpty()
			Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             currentZone,
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to deployment once placement is done", func() {
			applicationResource := makeApplication()
			existingCondition := v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             currentZone,
				Status:             string(v1.PlacementStatusDone),
				LastTransitionTime: fakeClock.NowTime(),
			}
			applicationResource.Status.State = v1.PlacementGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}
			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions:  []v1.ConditionStatus{existingCondition},
				},
			}
			existingJobCondition := types.FromCondition(existingCondition, types.AsyncJobTypeLocalPlacement)

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version), mo.None[*types.SpecificVersion](),
				fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.PlacementConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.PlacementStatusDone),
									LastTransitionTime: fakeClock.NowTime(),
								},
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusPull),
				LastTransitionTime: fakeClock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to operational once deployment is completed ", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.RelocationGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}
			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.PlacementStatusDone),
							LastTransitionTime: fakeClock.NowTime(),
						},
						{
							Type:               v1.DeploymentConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.DeploymentStatusDone),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.FromCondition(v1.ConditionStatus{
				Type:               v1.DeploymentConditionType,
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusDone),
				LastTransitionTime: fakeClock.NowTime(),
			}, types.AsyncJobTypeDeploy)

			localApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, true)
			localApplications := map[types.SpecificVersion]*local.LocalApplication{*version: &localApp}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeTrue())
			Expect(globalApplication.IsPresent()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.OperationalGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.PlacementConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.PlacementStatusDone),
									LastTransitionTime: fakeClock.NowTime(),
								},
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.DeploymentStatusDone),
									LastTransitionTime: fakeClock.NowTime(),
								},
								{
									Type:               v1.LocalConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("should relocate if placements has changed", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      "otherzone",
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.LocalConditionType,
							ZoneId:             "otherzone",
							Status:             string(health.HealthStatusProgressing),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      "otherzone",
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.LocalConditionType,
									ZoneId:             "otherzone",
									Status:             string(health.HealthStatusProgressing),
									LastTransitionTime: fakeClock.NowTime(),
								},
							},
						},
						{
							ZoneId:      currentZone,
							ZoneVersion: 0,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusPull),
				LastTransitionTime: fakeClock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("should undeploy if current zone not in placements anymore", func() {
			applicationResource := makeApplication()
			localJobCondition := v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             currentZone,
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			}
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: "otherzone"}}
			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions:  []v1.ConditionStatus{localJobCondition},
				},
				{
					ZoneId:      "otherzone",
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.LocalConditionType,
							ZoneId:             "otherzone",
							Status:             string(health.HealthStatusProgressing),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.FromCondition(localJobCondition, types.AsyncJobTypeLocalOperation)

			localApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, true)
			localApplications := map[types.SpecificVersion]*local.LocalApplication{
				*version: &localApp,
			}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeTrue())
			Expect(globalApplication.IsPresent()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.OperationalGlobalState,
					Placements: []v1.Placement{
						{Zone: "otherzone"},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.UndeploymentConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.UndeploymentStatusUndeploy),
									LastTransitionTime: fakeClock.NowTime(),
								},
							},
						},
						{
							ZoneId:      "otherzone",
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.LocalConditionType,
									ZoneId:             "otherzone",
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
				Type:               v1.UndeploymentConditionType,
				ZoneId:             currentZone,
				Status:             string(v1.UndeploymentStatusUndeploy),
				LastTransitionTime: fakeClock.NowTime(),
			}))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to global failure if deployment job fails", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.DeploymentConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.DeploymentStatusFailure),
							LastTransitionTime: fakeClock.NowTime(),
							RetryAttempt:       1,
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.FailureGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.DeploymentStatusFailure),
									LastTransitionTime: fakeClock.NowTime(),
									RetryAttempt:       1,
								},
							},
						},
					},
				},
			))

			Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to operational state if undeployment job fails and application is deployed", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Owner = currentZone
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.UndeploymentConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.UndeploymentStatusFailure),
							LastTransitionTime: fakeClock.NowTime(),
							RetryAttempt:       1,
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, true)
			localApplications := map[types.SpecificVersion]*local.LocalApplication{*version: &localApp}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeTrue())
			Expect(globalApplication.IsPresent()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.OperationalGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.LocalConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

		})

		It("switch to operational state if undeployment job fails and application is not deployed", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Owner = currentZone
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.UndeploymentConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.UndeploymentStatusFailure),
							LastTransitionTime: fakeClock.NowTime(),
							RetryAttempt:       1,
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			fakeLocalApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, false)

			localApplications := map[types.SpecificVersion]*local.LocalApplication{*version: &fakeLocalApp}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusPull),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

		})

		It("switch to failure state if undeployment job fails and there are no placements", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Owner = currentZone
			applicationResource.Status.Placements = []v1.Placement{}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.UndeploymentConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.UndeploymentStatusFailure),
							LastTransitionTime: fakeClock.NowTime(),
							RetryAttempt:       1,
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, true)
			localApplications := map[types.SpecificVersion]*local.LocalApplication{*version: &localApp}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeTrue())
			Expect(globalApplication.IsPresent()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State:      v1.FailureGlobalState,
					Placements: []v1.Placement{},
					Owner:      currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.UndeploymentConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.UndeploymentStatusFailure),
									LastTransitionTime: fakeClock.NowTime(),
									RetryAttempt:       1,
								},
							},
						},
					},
				},
			))

			Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

		})

		It("undeployment in different zone fails", func() {

			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Owner = currentZone
			applicationResource.Status.Placements = []v1.Placement{
				{Zone: "otherzone"},
			}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      "otherzone",
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.UndeploymentConditionType,
							ZoneId:             "otherzone",
							Status:             string(v1.UndeploymentStatusFailure),
							LastTransitionTime: fakeClock.NowTime(),
							RetryAttempt:       1,
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.FailureGlobalState,
					Placements: []v1.Placement{
						{Zone: "otherzone"},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      "otherzone",
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.UndeploymentConditionType,
									ZoneId:             "otherzone",
									Status:             string(v1.UndeploymentStatusFailure),
									LastTransitionTime: fakeClock.NowTime(),
									RetryAttempt:       1,
								},
							},
						},
					},
				},
			))

			Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

		})

		It("switch to deployment if operational job finds missing resources", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.LocalConditionType,
							ZoneId:             currentZone,
							Status:             string(health.HealthStatusMissing),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.None[*types.SpecificVersion](), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:      currentZone,
							ZoneVersion: 1,
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusPull),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

		})

		It("should set initial version in status", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.PlacementGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.PlacementConditionType,
							ZoneId:             currentZone,
							Status:             string(v1.PlacementStatusDone),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApplications := make(map[types.SpecificVersion]*local.LocalApplication)
			globalApplication := NewFromLocalApplication(localApplications, mo.None[*types.SpecificVersion](),
				mo.Some(newVersion), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeFalse())
			Expect(globalApplication.IsPresent()).To(BeFalse())
			Expect(globalApplication.IsVersionChanged()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()

			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:       currentZone,
							ZoneVersion:  1,
							ChartVersion: newVersion.ToString(),
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.PlacementConditionType,
									ZoneId:             currentZone,
									Status:             string(v1.PlacementStatusDone),
									LastTransitionTime: fakeClock.NowTime(),
								},
								{
									Type:               v1.DeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.DeploymentStatusPull),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

		It("switch to deployment if operational job finds new version", func() {
			applicationResource := makeApplication()
			applicationResource.Status.State = v1.OperationalGlobalState
			applicationResource.Status.Placements = []v1.Placement{{Zone: currentZone}}

			applicationResource.Status.Zones = []v1.ZoneStatus{
				{
					ZoneId:      currentZone,
					ZoneVersion: 1,
					Conditions: []v1.ConditionStatus{
						{
							Type:               v1.LocalConditionType,
							ZoneId:             currentZone,
							Status:             string(health.HealthStatusMissing),
							LastTransitionTime: fakeClock.NowTime(),
						},
					},
				},
			}

			existingJobCondition := types.EmptyJobConditions()

			localApp := local.FakeLocalApplication(runtimeConfig, version, fakeClock, true)
			localApplications := map[types.SpecificVersion]*local.LocalApplication{*version: &localApp}
			globalApplication := NewFromLocalApplication(localApplications, mo.Some(version),
				mo.Some(newVersion), fakeClock, applicationResource, runtimeConfig, logf.Log)

			Expect(globalApplication.IsDeployed()).To(BeTrue())
			Expect(globalApplication.IsPresent()).To(BeTrue())
			Expect(globalApplication.IsVersionChanged()).To(BeTrue())

			statusResult := globalApplication.DeriveNewStatus(existingJobCondition, jobFactory)

			status := statusResult.Status.OrEmpty()
			jobs := statusResult.Jobs

			Expect(status).To(Equal(
				v1.AnyApplicationStatus{
					State: v1.RelocationGlobalState,
					Placements: []v1.Placement{
						{Zone: currentZone},
					},
					Owner: currentZone,
					Zones: []v1.ZoneStatus{
						{
							ZoneId:       currentZone,
							ZoneVersion:  1,
							ChartVersion: newVersion.ToString(),
							Conditions: []v1.ConditionStatus{
								{
									Type:               v1.UndeploymentConditionType,
									ZoneId:             currentZone,
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
				ZoneId:             currentZone,
				Status:             string(v1.UndeploymentStatusUndeploy),
				LastTransitionTime: fakeClock.NowTime(),
			}))

			Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))
		})

	})

})

func makeApplication() *v1.AnyApplication {
	application := &v1.AnyApplication{
		ObjectMeta: metav1.ObjectMeta{
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
			Owner: CURRENT_ZONE,
		},
	}

	return application

}
