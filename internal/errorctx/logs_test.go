package errorctx

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("RealLogFetcher", func() {
	var (
		fakeClient *k8sfake.Clientset
		fetcher    *logFetcher
		ctx        context.Context
		logData    = "fake logs"
		eventData  *corev1.EventList
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = k8sfake.NewSimpleClientset()
		eventData = &corev1.EventList{
			Items: []corev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event",
						Namespace: "default",
					},
					Reason:  "TestReason",
					Message: "This is a test event",
				},
			},
		}

		fakeClient.Fake.PrependReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return false, nil, nil // Allow GetLogs to continue
		})

		fakeClient.Fake.PrependReactor("list", "events", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, eventData, nil
		})

		fetcher = NewRealLogFetcher(fakeClient)
	})

	It("should return log contents successfully", func() {
		log, err := fetcher.FetchLogs(ctx, "default", "test-pod", "test-container", false)

		Expect(err).NotTo(HaveOccurred())
		Expect(log).To(Equal(logData))
	})

	It("should return event contents successfully", func() {
		events, err := fetcher.FetchEvents(ctx, "default")

		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(Equal(eventData))
	})

})
