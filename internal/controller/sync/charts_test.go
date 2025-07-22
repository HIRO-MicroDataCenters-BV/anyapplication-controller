package sync

import (
	// 	"context"
	// 	"fmt"
	// 	"time"

	// 	"github.com/argoproj/gitops-engine/pkg/cache"
	// 	"github.com/argoproj/gitops-engine/pkg/health"
	// 	"github.com/argoproj/gitops-engine/pkg/sync/common"
	// 	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	. "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"
	// "helm.sh/helm/v3/pkg/chartutil"
	// v1 "hiro.io/anyapplication/api/v1"
	// "hiro.io/anyapplication/internal/clock"
	// "hiro.io/anyapplication/internal/config"
	// "hiro.io/anyapplication/internal/controller/fixture"
	// "hiro.io/anyapplication/internal/controller/types"
	// "hiro.io/anyapplication/internal/helm"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	// "k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/client-go/rest"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/client/fake"
	// logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Charts", func() {
	// var (
	// 	fakeClock     clock.Clock
	// 	syncManager   types.SyncManager
	// 	kubeClient    client.Client
	// 	helmClient    helm.HelmClient
	// 	application   *v1.AnyApplication
	// 	scheme        *runtime.Scheme
	// 	clusterCache  cache.ClusterCache
	// 	runtimeConfig config.ApplicationRuntimeConfig
	// 	gitOpsEngine  *fixture.FakeGitOpsEngine
	// 	updateFuncs   []cache.UpdateSettingsFunc
	// )

	// BeforeEach(func() {
	// 	fakeClock = clock.NewFakeClock()
	// 	scheme = runtime.NewScheme()
	// 	_ = v1.AddToScheme(scheme)

	// 	runtimeConfig = config.ApplicationRuntimeConfig{
	// 		ZoneId:                        "zone",
	// 		PollOperationalStatusInterval: 100 * time.Millisecond,
	// 	}

	// 	application = &v1.AnyApplication{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      "test-app",
	// 			Namespace: "default",
	// 		},
	// 		Spec: v1.AnyApplicationSpec{
	// 			Source: v1.ApplicationSourceSpec{
	// 				HelmSelector: &v1.ApplicationSourceHelm{
	// 					Repository: "https://helm.nginx.com/stable",
	// 					Chart:      "nginx-ingress",
	// 					Version:    "2.0.1",
	// 					Namespace:  "default",
	// 				},
	// 			},
	// 			Zones: 1,
	// 			PlacementStrategy: v1.PlacementStrategySpec{
	// 				Strategy: v1.PlacementStrategyLocal,
	// 			},
	// 			RecoverStrategy: v1.RecoverStrategySpec{},
	// 		},
	// 		Status: v1.AnyApplicationStatus{
	// 			Owner: "zone",
	// 			State: v1.PlacementGlobalState,
	// 		},
	// 	}

	// 	application = application.DeepCopy()
	// })

	// BeforeEach(func() {
	// 	config := &rest.Config{
	// 		Host: "https://test",
	// 	}
	// 	options := helm.HelmClientOptions{
	// 		RestConfig: config,
	// 		KubeVersion: &chartutil.KubeVersion{
	// 			Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
	// 			Major:   "1",
	// 			Minor:   "23",
	// 		},
	// 	}
	// 	helmClient, _ = helm.NewHelmClient(&options)

	// 	kubeClient = fake.NewClientBuilder().
	// 		WithScheme(scheme).
	// 		WithStatusSubresource(&v1.AnyApplication{}).
	// 		Build()

	// 	updateFuncs = []cache.UpdateSettingsFunc{
	// 		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
	// 			info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
	// 			cacheManifest = true
	// 			return
	// 		}),
	// 	}

	// 	clusterCache, _ = fixture.NewTestClusterCacheWithOptions(updateFuncs)
	// 	if err := clusterCache.EnsureSynced(); err != nil {
	// 		Fail("Failed to sync cluster cache: " + err.Error())
	// 	}

	// })

	It("should return unique instance id for helm chart version and release", func() {
	})

})
