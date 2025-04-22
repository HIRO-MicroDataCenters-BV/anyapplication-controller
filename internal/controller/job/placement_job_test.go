package job

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Placement Job", func() {
	Context("When created", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		applicationResource := makeApplication()
		clock := clock.NewFakeClock()

		It("should return starting condition", func() {
			placementJob := NewLocalPlacementJob(applicationResource, runtimeConfig, clock)

			Expect(placementJob.GetStatus()).To(Equal(v1.ConditionStatus{
				Type:               v1.PlacementConditionType,
				ZoneId:             "zone",
				Status:             string(v1.PlacementStatusInProgress),
				LastTransitionTime: clock.NowTime(),
			},
			))
		})

		It("should run and apply status", func() {
			placementJob := NewLocalPlacementJob(applicationResource, runtimeConfig, clock)

			context := NewAsyncJobContext()
			placementJob.Run(context)
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
