package sync

import (
	"context"
	"fmt"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sync Suite")
}

// func loadKubeConfig() (*rest.Config, error) {
// 	if home := homedir.HomeDir(); home != "" {
// 		kubeconfig := filepath.Join(home, ".kube", "config")
// 		return clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	}
// 	return rest.InClusterConfig()
// }

var _ = Describe("SyncManager", func() {
	var (
		syncManager  SyncManager
		kubeClient   client.Client
		helmClient   helm.HelmClient
		application  *v1.AnyApplication
		scheme       *runtime.Scheme
		clusterCache cache.ClusterCache
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
						Namespace:  "nginx",
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

		application = application.DeepCopy()
	})

	// BeforeEach(func() {
	// 	config, _ := loadKubeConfig()

	// 	mgr, err := ctrl.NewManager(config, ctrl.Options{})
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	kubeClient = mgr.GetClient()

	// 	options := helm.HelmClientOptions{
	// 		RestConfig: config,
	// 		KubeVersion: &chartutil.KubeVersion{
	// 			Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
	// 			Major:   "1",
	// 			Minor:   "23",
	// 		},
	// 	}
	// 	helmClient, err = helm.NewHelmClient(&options)
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	clusterCache = cache.NewClusterCache(config)
	// 	syncManager = NewSyncManager(kubeClient, helmClient, clusterCache)
	// })

	BeforeEach(func() {
		options := helm.HelmClientOptions{
			RestConfig: &rest.Config{Host: "https://test"},
			KubeVersion: &chartutil.KubeVersion{
				Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
				Major:   "1",
				Minor:   "23",
			},
		}
		helmClient, _ = helm.NewHelmClient(&options)

		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		clusterCache = NewTestClusterCacheWithOptions([]cache.UpdateSettingsFunc{})
		syncManager = NewSyncManager(kubeClient, helmClient, clusterCache)

	})

	It("should sync helm release", func() {
		syncResult, err := syncManager.Sync(context.Background(), application)

		Expect(err).NotTo(HaveOccurred())
		Expect(syncResult.Total).To(Equal(23))
		Expect(syncResult.Created).To(Equal(23))
		Expect(syncResult.CreateFailed).To(Equal(0))
		Expect(syncResult.Updated).To(Equal(0))
		Expect(syncResult.UpdateFailed).To(Equal(0))
		Expect(syncResult.Deleted).To(Equal(0))
		Expect(syncResult.DeleteFailed).To(Equal(0))
		Expect(syncResult.Status.Status).To(Equal(health.HealthStatusProgressing))
	})

	It("should delete helm release", func() {
		_, err := syncManager.Sync(context.Background(), application)
		Expect(err).NotTo(HaveOccurred())

		syncResult, err := syncManager.Delete(context.TODO(), application)

		Expect(err).NotTo(HaveOccurred())
		Expect(syncResult.Total).To(Equal(23))
		Expect(syncResult.Created).To(Equal(0))
		Expect(syncResult.CreateFailed).To(Equal(0))
		Expect(syncResult.Updated).To(Equal(0))
		Expect(syncResult.UpdateFailed).To(Equal(0))
		Expect(syncResult.Deleted).To(Equal(23))
		Expect(syncResult.DeleteFailed).To(Equal(0))
		Expect(syncResult.Status.Status).To(Equal(health.HealthStatusMissing))
	})

})
