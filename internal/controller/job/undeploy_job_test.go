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

var _ = Describe("UndeployJob", func() {
	var (
		undeployJob *UndeployJob
		application *v1.AnyApplication
		scheme      *runtime.Scheme
	)

	BeforeEach(func() {
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

		undeployJob = NewUndeployJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)

	})

	It("should return initial status", func() {
		status := undeployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(v1.ConditionStatus{
			Type:               v1.UndeploymentConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: metav1.Time{},
		},
		))

		Expect(undeployJob.GetJobID()).To(Equal(types.JobId{
			JobType: types.AsyncJobTypeUndeploy,
			ApplicationId: types.ApplicationId{
				Name:      application.Name,
				Namespace: application.Namespace,
			},
		}))

	})

	It("should run and apply done status", func() {
		jobContextDeploy, cancelDeploy := jobContext.WithCancel()
		version, _ := types.NewSpecificVersion("2.0.1")
		deployJob := NewDeployJob(application, version, &runtimeConfig, theClock, logf.Log, &fakeEvents)
		go deployJob.Run(jobContextDeploy)

		waitForJobStatus(deployJob, string(v1.DeploymentStatusDone))
		cancelDeploy()

		jobContextUndeploy, cancelUndeploy := jobContext.WithCancel()
		defer cancelUndeploy()

		go undeployJob.Run(jobContextUndeploy)

		waitForJobStatus(undeployJob, string(v1.UndeploymentStatusDone))

		status := undeployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.UndeploymentConditionType,
				ZoneId:             "zone",
				Status:             string(v1.UndeploymentStatusDone),
				Msg:                "Undeploy state changed to 'Done'.Version 2.0.1 (Total=23, Deleted=23, DeleteFailed=0). ",
				LastTransitionTime: metav1.Time{},
			},
		))

	})

})
