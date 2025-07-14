package errorctx

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("RealLogFetcher", func() {
	var (
		fakeClient *k8sfake.Clientset
		fetcher    *RealLogFetcher
		ctx        context.Context
		logData    = "fake logs"
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = k8sfake.NewSimpleClientset()

		// Inject log stream mock into the reactor
		fakeClient.Fake.PrependReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return false, nil, nil // Allow GetLogs to continue
		})

		fetcher = NewRealLogFetcher(fakeClient)
	})

	It("should return log contents successfully", func() {
		log, err := fetcher.FetchLogs(ctx, "default", "test-pod", "test-container", false)

		Expect(err).NotTo(HaveOccurred())
		Expect(log).To(Equal(logData))
	})
})
