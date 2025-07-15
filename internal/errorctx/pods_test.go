package errorctx

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/controller/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("K8sReportService", func() {
	var (
		fakeClient *k8sfake.Clientset
		reporter   *K8sReportService
		pod        corev1.Pod
		scheme     *runtime.Scheme
		logOutput  = "fake logs"
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)

		pod = corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"dcp.hiro.io/instance-id": "instanceId",
					"dcp.hiro.io/managed-by":  "dcp",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
				UID:               "test-pod-uid",
				ResourceVersion:   "1",
			},
			Spec: corev1.PodSpec{
				NodeName: "test-node",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "main",
						RestartCount: 1,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
						},
					},
				},
			},
		}

		// Fake client with pod and event
		fakeClient = k8sfake.NewClientset(
			&pod,
			&corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-event",
					Namespace: "default",
				},
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod",
					Namespace: "default",
				},
				Reason:         "BackOff",
				Message:        "Back-off restarting failed container",
				Type:           "Warning",
				FirstTimestamp: metav1.NewTime(time.Now()),
			},
		)

		clusterCache := fixture.NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
				cacheManifest = true
				return
			}),
		}, &pod)
		clusterCache.EnsureSynced()

		logFetcher := NewRealLogFetcher(fakeClient)
		reporter = NewK8sReportService(clusterCache, logFetcher)
	})

	It("should generate a pod report with logs and events", func() {

		report, err := reporter.GeneratePodReport(context.TODO(), "instanceId", "default")
		Expect(err).NotTo(HaveOccurred())
		Expect(report.Pods).To(HaveLen(1))

		p := report.Pods[0]
		Expect(p.Name).To(Equal("test-pod"))
		Expect(p.Status).To(Equal("CrashLoopBackOff"))
		Expect(p.Restarts).To(Equal(int32(1)))

		Expect(p.Logs).To(HaveLen(1))
		Expect(p.Logs[0].Log).To(Equal(logOutput))

		Expect(p.Events).NotTo(BeEmpty())
		Expect(p.Events[0].Reason).To(Equal("BackOff"))
	})
})
