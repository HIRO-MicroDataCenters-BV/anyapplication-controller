package job

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("LocalOperationJob", func() {
	var (
		operationJob *LocalOperationJob
		application  *v1.AnyApplication
		scheme       *runtime.Scheme
		version      *types.SpecificVersion
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
				Owner: "zone",
				State: v1.PlacementGlobalState,
			},
		}

		operationJob = NewLocalOperationJob(application, &runtimeConfig, theClock, logf.Log, &fakeEvents)
	})

	It("should return initial status", func() {
		status := operationJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(health.HealthStatusProgressing),
			LastTransitionTime: metav1.Time{},
		},
		))

		Expect(operationJob.GetJobID()).To(Equal(types.JobId{
			JobType: types.AsyncJobTypeLocalOperation,
			ApplicationId: types.ApplicationId{
				Name:      application.Name,
				Namespace: application.Namespace,
			},
		}))

	})

	It("should sync periodically and report status", func() {

		deployJob := NewDeployJob(application, version, &runtimeConfig, theClock, logf.Log, &fakeEvents)
		deployJobContext, cancelDeploy := jobContext.WithCancel()

		go deployJob.Run(deployJobContext)

		waitForJobStatus(deployJob, string(v1.DeploymentStatusDone))
		cancelDeploy()

		operationJobContext, cancelOperation := jobContext.WithCancel()
		defer cancelOperation()

		go operationJob.Run(operationJobContext)

		waitForJobStatus(operationJob, string(health.HealthStatusProgressing))

		status := operationJob.GetStatus()
		status.LastTransitionTime = metav1.Time{}

		Expect(status).To(Equal(
			v1.ConditionStatus{
				Type:               v1.LocalConditionType,
				ZoneId:             "zone",
				Status:             string(health.HealthStatusProgressing),
				LastTransitionTime: metav1.Time{},
				Msg:                "",
			},
		))

	})

})
