package local

import (
	"reflect"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"hiro.io/anyapplication/internal/controller/fixture"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestApplicationBundleSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ApplicationBundleSuite")
}

var _ = Describe("ApplicationBundle", func() {

	It("application bundle with empty resources should be healthy", func() {

		bundle, _ := LoadApplicationBundle(nil, nil, logf.Log)

		Expect(bundle.IsDeployed()).To(BeTrue())

		state, msg, err := bundle.DetermineState()
		Expect(err).ToNot(HaveOccurred())
		Expect(state).To(Equal(health.HealthStatusHealthy))
		Expect(msg).To(BeEmpty())

	})

	It("application bundle with all deployed resources is deployed and healthy", func() {
		bundle := fixture.LoadJSONFixture[ApplicationBundle]("application_bundle.json")

		Expect(bundle.IsDeployed()).To(BeTrue())

		state, msg, err := bundle.DetermineState()
		Expect(err).ToNot(HaveOccurred())
		Expect(state).To(Equal(health.HealthStatusHealthy))
		Expect(msg).To(BeEmpty())

	})

	It("application bundle with partially deployed resources is not deployed and not healthy", func() {
		bundle := fixture.LoadJSONFixture[ApplicationBundle]("application_bundle.json")

		Expect(bundle.IsDeployed()).To(BeTrue())
		Expect(bundle.availableResources).To(HaveLen(2))

		bundle.availableResources = bundle.availableResources[:1] // Simulate partial deployment

		Expect(bundle.IsDeployed()).To(BeFalse())

		state, msg, err := bundle.DetermineState()
		Expect(err).ToNot(HaveOccurred())
		Expect(state).To(Equal(health.HealthStatusMissing))
		Expect(msg).To(Equal([]string{"Resource is missing: apps/v1, Kind=Deployment kube-system/coredns"}))

	})

	It("application bundle can be serialized and deserialized", func() {
		expected := fixture.LoadJSONFixture[ApplicationBundle]("application_bundle.json")
		serialized, _ := expected.Serialize()
		actual, _ := Deserialize(serialized)
		Expect(reflect.DeepEqual(actual, expected)).To(BeTrue(), "Serialization/Deserialization error")
	})

})
