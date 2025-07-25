package job

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var _ = Describe("UndeployJobUnitests", func() {
	var (
		undeployJob    *UndeployJob
		application    *v1.AnyApplication
		scheme         *runtime.Scheme
		gitOpsEngine   *fixture.FakeGitOpsEngine
		kubeClient     client.Client
		fakeClock      *clock.FakeClock
		jobContext     types.AsyncJobContext
		runtimeConfig  config.ApplicationRuntimeConfig
		fakeHelmClient *helm.FakeHelmClient
		updateFuncs    []cache.UpdateSettingsFunc
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = v1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		fakeClock = clock.NewFakeClock()
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

		pod := corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					sync.LABEL_INSTANCE_ID:   "default-test-app",
					sync.LABEL_CHART_VERSION: "2.0.0",
					sync.LABEL_MANAGED_BY:    "dcp",
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

		gitOpsEngine = fixture.NewFakeGitopsEngine()

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application, &pod).
			WithStatusSubresource(&v1.AnyApplication{}).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					return fmt.Errorf("delete error")
				},
			}).
			Build()

		fakeHelmClient = helm.NewFakeHelmClient()

		yamlData, err := yaml.Marshal(pod)
		if err != nil {
			panic(err)
		}
		fakeHelmClient.MockTemplate(string(yamlData))

		runtimeConfig = config.ApplicationRuntimeConfig{
			ZoneId:                        "zone",
			PollOperationalStatusInterval: 100 * time.Millisecond,
			PollSyncStatusInterval:        300 * time.Millisecond,
			DefaultSyncTimeout:            300 * time.Millisecond,
		}
		updateFuncs = []cache.UpdateSettingsFunc{
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
				cacheManifest = true
				return
			}),
		}

		clusterCache, _ := fixture.NewTestClusterCacheWithOptions(updateFuncs, &pod)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		charts := sync.NewCharts(context.TODO(), helmClient, &sync.ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)

		applications := sync.NewApplications(kubeClient, fakeHelmClient,
			charts, clusterCache, fakeClock, &runtimeConfig, gitOpsEngine, logf.Log)

		jobContext = NewAsyncJobContext(fakeHelmClient, kubeClient, ctx, applications)

		undeployJob = NewUndeployJob(application, &runtimeConfig, fakeClock, logf.Log, &fakeEvents)
	})

	It("should retry and fail after several attempts", func() {
		jobContext, cancel := jobContext.WithCancel()
		defer cancel()

		go undeployJob.Run(jobContext)

		fakeClock.Advance(1 * time.Second)

		status := undeployJob.GetStatus()
		Expect(status.Status).To(Equal(string(v1.UndeploymentStatusUndeploy)))
		waitForJobMsg(undeployJob, "Undeploy failure: Undeployment timed out Retrying undeployment (attempt 2 of 3).")

		fakeClock.Advance(1 * time.Second)

		status = undeployJob.GetStatus()
		Expect(status.Status).To(Equal(string(v1.UndeploymentStatusUndeploy)))
		waitForJobMsg(undeployJob, "Undeploy failure: Undeployment timed out Retrying undeployment (attempt 3 of 3).")

		fakeClock.Advance(1 * time.Second)

		waitForJobStatus(undeployJob, string(v1.UndeploymentStatusFailure))

		Expect(undeployJob.GetStatus().Msg).To(Equal("Undeploy failure: Failure after 3 attempts."))
	})

	It("should undeploy multiple versions", func() {
		Fail("This test is not implemented yet")
	})

})

type FakeObjectTracker struct {
}

func NewObjectTracker() *FakeObjectTracker {
	return &FakeObjectTracker{}
}

func (f *FakeObjectTracker) Add(obj runtime.Object) error {
	return nil
}

// Get retrieves the object by its kind, namespace and name.
func (f *FakeObjectTracker) Get(gvr schema.GroupVersionResource, ns, name string, opts ...metav1.GetOptions) (runtime.Object, error) {
	return nil, nil
}
func (f *FakeObjectTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.CreateOptions) error {
	return nil
}

// Update updates an existing object in the tracker in the specified namespace.
func (f *FakeObjectTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.UpdateOptions) error {
	return nil
}

// Patch patches an existing object in the tracker in the specified namespace.
func (f *FakeObjectTracker) Patch(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	return nil
}

// Apply applies an object in the tracker in the specified namespace.
func (f *FakeObjectTracker) Apply(gvr schema.GroupVersionResource, applyConfiguration runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	return nil
}

// List retrieves all objects of a given kind in the given
// namespace. Only non-List kinds are accepted.
func (f *FakeObjectTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string, opts ...metav1.ListOptions) (runtime.Object, error) {
	return nil, nil
}

// Delete deletes an existing object from the tracker. If object
// didn't exist in the tracker prior to deletion, Delete returns
// no error.
func (f *FakeObjectTracker) Delete(gvr schema.GroupVersionResource, ns, name string, opts ...metav1.DeleteOptions) error {
	return fmt.Errorf("delete error")
}

// Watch watches objects from the tracker. Watch returns a channel
// which will push added / modified / deleted object.
func (f *FakeObjectTracker) Watch(gvr schema.GroupVersionResource, ns string, opts ...metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}
