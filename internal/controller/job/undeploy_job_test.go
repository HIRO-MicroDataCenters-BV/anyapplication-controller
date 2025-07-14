package job

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
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
				Application: v1.ApplicationMatcherSpec{
					HelmSelector: &v1.HelmSelectorSpec{
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
				Owner: "zone",
				State: v1.PlacementGlobalState,
			},
		}

		undeployJob = NewUndeployJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)

	})

	It("should return initial status", func() {
		status := undeployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(v1.ConditionStatus{
			Type:               v1.UndeploymenConditionType,
			ZoneId:             "zone",
			Status:             string(v1.UndeploymentStatusUndeploy),
			LastTransitionTime: metav1.Time{},
		},
		))
	})

	It("should run and apply done status", func() {

		deployJob := NewDeployJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)
		deployJob.Run(jobContext)

		waitForJobStatus(deployJob, string(v1.DeploymentStatusDone))

		undeployJob.Run(jobContext)

		waitForJobStatus(undeployJob, string(v1.UndeploymentStatusDone))

		status := undeployJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.UndeploymenConditionType,
				ZoneId:             "zone",
				Status:             string(v1.UndeploymentStatusDone),
				LastTransitionTime: metav1.Time{},
			},
		))
		undeployJob.Stop()

	})

})
