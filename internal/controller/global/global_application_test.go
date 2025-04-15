package global

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/local"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GlobalApplication", func() {
	Context("When new resource is created", func() {
		runtimeConfig := &config.ApplicationRuntimeConfig{
			ZoneId: "zone",
		}
		localApplication := mo.None[local.LocalApplication]()
		applicationResource := makeApplication()
		globalApplication := NewFromLocalApplication(localApplication, applicationResource, runtimeConfig)

		It("transit to placement state", func() {
			status := globalApplication.DeriveNewStatus(EmptyJobConditions())
			// statusOpt := status.OrElse(v1.AnyApplicationStatus{})
			Expect(status.IsAbsent()).To(BeTrue())
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
			Zones:           1,
			RecoverStrategy: v1.RecoverStrategySpec{},
		},
	}

	return application

}
