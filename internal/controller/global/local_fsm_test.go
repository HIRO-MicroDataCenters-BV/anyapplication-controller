// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

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
		localApplications map[types.SpecificVersion]*local.LocalApplication
		globalApplication types.GlobalApplication
		fakeEvents        events.Events
		version100        *types.SpecificVersion
		newVersion010     *types.SpecificVersion
	)

	BeforeEach(func() {
		fakeClock = clock.NewFakeClock()
		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		fakeEvents = events.NewFakeEvents()
		jobFactory = job.NewAsyncJobFactory(&runtimeConfig, fakeClock, logf.Log, &fakeEvents)
		localApplications = make(map[types.SpecificVersion]*local.LocalApplication)

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
				Ownership: v1.OwnershipStatus{
					Epoch: 1,
					Owner: "otherzone",
					State: v1.UnknownGlobalState,
				},
			},
		}
		version100, _ = types.NewSpecificVersion("1.0.0")
		newVersion010, _ = types.NewSpecificVersion("0.1.0")
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

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
		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}

		Expect(globalApplication.IsDeployed()).To(BeFalse())
		Expect(globalApplication.IsPresent()).To(BeFalse())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
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

		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}
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
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
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
		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}

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

		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{
			*version100: &localApp,
		}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
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

		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}

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

		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{
			*version100: &localApp,
		}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.FromCondition(operationalCondition, types.AsyncJobTypeLocalOperation), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
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
		application.Status.Ownership.Placements = []v1.Placement{{Zone: "otherzone"}}

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

		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{*version100: &localApp}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.EmptyJobConditions(), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "otherzone"}},
				},
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

		application.Status.Ownership.Placements = []v1.Placement{{Zone: "otherzone"}}

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
		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{*version100: &localApp}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(types.FromCondition(undeployCondition, types.AsyncJobTypeUndeploy), jobFactory)

		status := statusResult.Status.OrEmpty()
		jobs := statusResult.Jobs
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "otherzone",
					State:      v1.UnknownGlobalState,
					Placements: []v1.Placement{{Zone: "otherzone"}},
				},
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

	It("switch to undeployment if operational job finds new version", func() {

		operationCondition := v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: fakeClock.NowTime(),
		}
		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}
		application.Status.Ownership.Owner = "zone"
		application.Status.Zones = []v1.ZoneStatus{
			{
				ZoneId:       "zone",
				ZoneVersion:  1,
				ChartVersion: "1.0.0",
				Conditions:   []v1.ConditionStatus{operationCondition},
			},
		}
		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{*version100: &localApp}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(version100),
			mo.Some(newVersion010), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeTrue())
		Expect(globalApplication.IsPresent()).To(BeTrue())
		Expect(globalApplication.NonActiveVersionsPresent()).To(BeFalse())
		Expect(globalApplication.IsVersionChanged()).To(BeTrue())

		statusResult := globalApplication.DeriveNewStatus(
			types.FromCondition(operationCondition, types.AsyncJobTypeLocalOperation), jobFactory,
		)
		status := statusResult.Status.OrEmpty()
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "zone",
					State:      v1.RelocationGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
				Zones: []v1.ZoneStatus{
					{
						ZoneId:       "zone",
						ZoneVersion:  1,
						ChartVersion: "0.1.0",
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
		jobs := statusResult.Jobs
		jobToAdd := jobs.JobsToAdd.OrEmpty()
		Expect(jobToAdd.GetStatus()).To(Equal(v1.ConditionStatus{
			Type:               v1.UndeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		}))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

	})

	It("let undeployment job to finish if new version is available", func() {

		operationCondition := v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: fakeClock.NowTime(),
		}
		undeploymentCondition := v1.ConditionStatus{
			Type:               v1.UndeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: fakeClock.NowTime(),
		}

		application.Status.Ownership.Placements = []v1.Placement{{Zone: "zone"}}
		application.Status.Ownership.Owner = "zone"
		application.Status.Zones = []v1.ZoneStatus{

			{
				ZoneId:       "zone",
				ZoneVersion:  1,
				ChartVersion: "0.1.0",
				Conditions:   []v1.ConditionStatus{operationCondition, undeploymentCondition},
			},
		}
		localApp := local.FakeLocalApplication(&runtimeConfig, version100, fakeClock, true)
		localApplications := map[types.SpecificVersion]*local.LocalApplication{*version100: &localApp}
		globalApplication = NewFromLocalApplication(localApplications, mo.Some(newVersion010),
			mo.None[*types.SpecificVersion](), fakeClock, &application, &runtimeConfig, logf.Log)

		Expect(globalApplication.IsDeployed()).To(BeFalse())
		Expect(globalApplication.IsPresent()).To(BeFalse())
		Expect(globalApplication.NonActiveVersionsPresent()).To(BeTrue())
		Expect(globalApplication.IsVersionChanged()).To(BeFalse())

		statusResult := globalApplication.DeriveNewStatus(
			types.FromCondition(undeploymentCondition, types.AsyncJobTypeUndeploy), jobFactory,
		)
		status := statusResult.Status.OrEmpty()
		Expect(status).To(Equal(
			v1.AnyApplicationStatus{
				Ownership: v1.OwnershipStatus{
					Epoch:      1,
					Owner:      "zone",
					State:      v1.RelocationGlobalState,
					Placements: []v1.Placement{{Zone: "zone"}},
				},
				Zones: []v1.ZoneStatus{
					{
						ZoneId:       "zone",
						ZoneVersion:  1,
						ChartVersion: "0.1.0",
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
		jobs := statusResult.Jobs
		Expect(jobs.JobsToAdd).To(Equal(mo.None[types.AsyncJob]()))
		Expect(jobs.JobsToRemove).To(Equal(mo.None[types.AsyncJobType]()))

	})

})
