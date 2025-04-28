package sync

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sync Suite")
}

func loadKubeConfig() (*rest.Config, error) {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

var _ = Describe("SyncManager", func() {
	var (
		syncManager SyncManager
		kubeClient  client.Client
		helmClient  helm.HelmClient
		application *v1.AnyApplication
		scheme      *runtime.Scheme
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

		// runtimeConfig = config.ApplicationRuntimeConfig{
		// 	ZoneId: "zone",
		// }

		// fakeClock = clock.NewFakeClock()

		// helmClient = helm.NewFakeHelmClient()

		// kubeClient = fake.NewClientBuilder().
		// 	WithScheme(scheme).
		// 	WithRuntimeObjects(application).
		// 	WithStatusSubresource(&v1.AnyApplication{}).
		// 	Build()

		config, _ := loadKubeConfig()

		// fmt.Println(config)

		mgr, err := ctrl.NewManager(config, ctrl.Options{})
		if err != nil {
			panic(err)
		}

		kubeClient = mgr.GetClient()

		options := helm.HelmClientOptions{
			RestConfig: config,
			KubeVersion: &chartutil.KubeVersion{
				Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
				Major:   "1",
				Minor:   "23",
			},
		}
		helmClient, err = helm.NewHelmClient(&options)
		if err != nil {
			panic(err)
		}

		application = application.DeepCopy()
		syncManager = NewSyncManager(kubeClient, helmClient, config)
	})

	It("should sync helm release", func() {
		go syncManager.InvalidateCache()
		time.Sleep(3 * time.Second)

		// status, err := syncManager.Sync(context.Background(), application)

		// Expect(err).NotTo(HaveOccurred())
		// fmt.Printf("%s \n", status)

		// status, _ = syncManager.Sync(context.Background(), application)
		// fmt.Printf("%s \n", status)

		// status, _ = syncManager.Sync(context.Background(), application)
		// fmt.Printf("%s \n", status)

	})

	// It("should delete helm release", func() {
	// 	err := syncManager.Delete(context.TODO(), application)

	// 	Expect(err).NotTo(HaveOccurred())

	// 	// Expect(syncManager.GetStatus()).To(Equal(v1.ConditionStatus{
	// 	// 	Type:               v1.RelocationConditionType,
	// 	// 	ZoneId:             "zone",
	// 	// 	Status:             string(v1.RelocationStatusPull),
	// 	// 	LastTransitionTime: fakeClock.NowTime(),
	// 	// },
	// 	// ))
	// })

})
