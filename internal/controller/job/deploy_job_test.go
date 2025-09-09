// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package job

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DeployJob", func() {
	var (
		deployJob   *DeployJob
		application *v1.AnyApplication
		scheme      *runtime.Scheme
		version     *types.SpecificVersion
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)
		version, _ = types.NewSpecificVersion("2.0.1")

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
				Ownership: v1.OwnershipStatus{
					Epoch: 1,
					Owner: "zone",
					State: v1.PlacementGlobalState,
				},
			},
		}

		deployJob = NewDeployJob(application, version, &runtimeConfig, theClock, logf.Log, &fakeEvents)
	})

	It("should return initial status", func() {
		status := deployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}
		Expect(status).To(Equal(v1.ConditionStatus{
			Type:               v1.DeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.DeploymentStatusPull),
			LastTransitionTime: metav1.Time{},
		},
		))

		Expect(deployJob.GetJobID()).To(Equal(types.JobId{
			JobType: types.AsyncJobTypeDeploy,
			ApplicationId: types.ApplicationId{
				Name:      application.Name,
				Namespace: application.Namespace,
			},
		}))
	})

	It("Deployment should run and apply done status", func() {
		jobContext = NewAsyncJobContext(helmClient, k8sClient, ctx, applications)
		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		go deployJob.Run(jobContext)
		waitForJobStatus(deployJob, string(v1.DeploymentStatusDone))

		status := deployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.DeploymentConditionType,
				ZoneId:             "zone",
				Status:             string(v1.DeploymentStatusDone),
				LastTransitionTime: metav1.Time{},
				Msg:                "Deployment state changed to 'Done'. ",
			},
		))

	})

	It("should sync report failure", func() {
		application.Spec.Source.HelmSelector = &v1.ApplicationSourceHelm{
			Repository: "test-repo",
			Chart:      "test-chart",
			Version:    "1.0.0",
		}
		version, _ := types.NewSpecificVersion("1.0.0")
		jobContext = NewAsyncJobContext(helmClient, k8sClient, ctx, applications)
		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		deployJob = NewDeployJob(application, version, &runtimeConfig, theClock, logf.Log, &fakeEvents)

		go deployJob.Run(jobContext)

		waitForJobStatus(deployJob, string(v1.DeploymentStatusFailure))

		status := deployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.DeploymentConditionType,
				ZoneId:             "zone",
				Status:             string(v1.DeploymentStatusFailure),
				LastTransitionTime: metav1.Time{},
				Msg:                "Deployment failure: Fail to render chart: Helm template failure: Failed to add or update chart repo: could not find protocol handler for: ",
				Reason:             "SyncError",
			},
		))

	})

})
