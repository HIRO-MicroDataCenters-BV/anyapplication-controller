package job

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client

	helmClient    helm.HelmClient
	theClock      clock.Clock
	runtimeConfig config.ApplicationRuntimeConfig
	jobContext    types.AsyncJobContext
	fakeEvents    events.Events
	applications  types.Applications
	stopFunc      engine.StopFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Job Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = dcpv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})

	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {

	pollSyncStatusInterval, _ := time.ParseDuration("3000ms")
	pollOperationalStatusInterval, _ := time.ParseDuration("1000ms")
	runtimeConfig = config.ApplicationRuntimeConfig{
		ZoneId:                        "zone",
		PollSyncStatusInterval:        pollSyncStatusInterval,
		PollOperationalStatusInterval: pollOperationalStatusInterval,
	}
	var err error
	helmClient, err = helm.NewHelmClient(&helm.HelmClientOptions{
		RestConfig: cfg,
		Debug:      false,
		Linting:    true,
		KubeVersion: &chartutil.KubeVersion{
			Version: "v1.23.10",
			Major:   "1",
			Minor:   "23",
		},
	})
	if err != nil {
		panic("error " + err.Error())
	}

	theClock = clock.NewClock()
	fakeEvents = events.NewFakeEvents()
	log := textlogger.NewLogger(textlogger.NewConfig())
	clusterCache := cache.NewClusterCache(cfg,
		cache.SetLogr(log),
		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
			managedByMark := un.GetLabels()["dcp.hiro.io/managed-by"]
			info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
			// cache resources that has that mark to improve performance
			cacheManifest = managedByMark != ""
			return
		}),
	)
	gitOpsEngine := engine.NewEngine(cfg, clusterCache, engine.WithLogr(log))
	stopFunc, err = gitOpsEngine.Run()
	if err != nil {
		panic("error " + err.Error())
	}
	charts := sync.NewCharts(context.TODO(), helmClient, &sync.ChartsOptions{SyncPeriod: 60 * time.Second}, logf.Log)
	applications = sync.NewApplications(k8sClient, helmClient, charts, clusterCache, theClock, &runtimeConfig, gitOpsEngine, logf.Log)

	jobContext = NewAsyncJobContext(helmClient, k8sClient, ctx, applications)
})

var _ = AfterEach(func() {
	stopFunc()
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func waitForJobStatus(job types.AsyncJob, status string) {
	for i := 0; i < 100; i++ {
		time.Sleep(300 * time.Millisecond)
		fmt.Printf("Waiting for job %s to reach status %s, current status: %s, condition: %v\n", job.GetJobID(), status, job.GetStatus().Status, job.GetStatus())
		if job.GetStatus().Status == status {
			return
		}
	}
	Fail(fmt.Sprintf("Expected status %s, but got %s, object %v\n", status, job.GetStatus().Status, job.GetStatus()))
}

func waitForJobMsg(job types.AsyncJob, msg string) {
	for i := 0; i < 100; i++ {
		time.Sleep(300 * time.Millisecond)
		fmt.Printf("Waiting for job %s to reach message %s, current message: %s, condition: %v\n", job.GetJobID(), msg, job.GetStatus().Msg, job.GetStatus())
		if job.GetStatus().Msg == msg {
			return
		}
	}
	Fail(fmt.Sprintf("Expected message %s, but got %s, object %v\n", msg, job.GetStatus().Msg, job.GetStatus()))
}
