package job

import (
	"context"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs", func() {
	var (
		ctx         context.Context
		fakeClient  client.Client
		application *v1.AnyApplication
		scheme      *runtime.Scheme
		fakeClock   clock.Clock
		// jobs        AsyncJobs
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeClock = clock.NewFakeClock()
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
				State: v1.PlacementGlobalState,
				Conditions: []v1.ConditionStatus{
					{
						Type:               v1.PlacementConditionType,
						ZoneId:             "zone",
						Status:             string(v1.PlacementStatusInProgress),
						LastTransitionTime: fakeClock.NowTime(),
					},
				},
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()

		// context := NewAsyncJobContext()
		// jobs = NewJobs(context)
	})

	It("should add a new condition if it does not exist", func() {
		newCondition := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone2",
			Status: string(v1.PlacementStatusDone),
		}

		err := AddOrUpdateStatusCondition(ctx, fakeClient, client.ObjectKeyFromObject(application), newCondition)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedApp.Status.Conditions).To(ContainElement(newCondition))
	})

})
