/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	configctrl "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/job"
	"hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/errorctx"
	"hiro.io/anyapplication/internal/helm"
	"hiro.io/anyapplication/internal/httpapi"
	"hiro.io/anyapplication/internal/resources"
	webhookdcpv1 "hiro.io/anyapplication/internal/webhook/v1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dcpv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var webhookServiceName string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPort int
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var configurationFile string
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The address the webhook endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate. If empty, certificates will be auto-generated in production mode.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "", "The webhook service name.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.StringVar(&configurationFile, "config", "/etc/dcp/application-controller.yaml",
		"Application Controller configuration file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	controllerConfig, err := config.LoadConfig(configurationFile)
	failIfError(err, setupLog, "Failed to load application configuration")
	applicationConfig := controllerConfig.Runtime

	loggers := make(map[string]logr.Logger)
	for name, levelStr := range controllerConfig.Logging.Components {
		lvl := config.ParseLevel(levelStr)
		loggers[name] = buildLogger(lvl).WithName(name)
	}
	// klog.InitFlags(nil)
	// flag.Set("v", "0") // or "2", etc. Higher values = more logs
	// flag.Parse()       // Bind flags if using Cobra or other CLI parsers
	// klog.SetOutput(io.Discard)

	logger := buildLogger(config.ParseLevel(controllerConfig.Logging.DefaultLevel))
	// logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Fetch base REST Configuration early to setup internal self-signed rotation if needed
	config, err := configctrl.GetConfigWithContext(applicationConfig.ZoneId)
	failIfError(err, setupLog, "unable to get config")

	// Get kubernetes clientset for configuration manipulations
	clientset, err := kubernetes.NewForConfig(config)
	failIfError(err, setupLog, "unable to create kubernetes clientset")

	// --- Automated In-Memory Webhook Certificate Generation Logic ---
	if len(webhookCertPath) == 0 && os.Getenv("ENABLE_WEBHOOKS") != "false" {
		webhookCertPath = "/tmp/k8s-webhook-server/serving-certs"

		generateAndPatchWebhookCertificates(clientset, setupLog, webhookCertPath, webhookCertName, webhookCertKey, webhookServiceName)
	}
	// -----------------------------------------------------------------

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		failIfError(err, setupLog, "Failed to initialize webhook certificate watcher")

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}
	webhookOptions := webhook.Options{TLSOpts: webhookTLSOpts}
	webhookOptions.Port = webhookPort

	webhookServer := webhook.NewServer(webhookOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)

		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "710ee37e.hiro.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	failIfError(err, setupLog, "unable to start manager")

	kubeClient := mgr.GetClient()
	helmClient, err := helm.NewHelmClient(&helm.HelmClientOptions{
		RestConfig: config,
		Debug:      false,
		Linting:    true,
		KubeVersion: &chartutil.KubeVersion{
			Version: "v1.23.10",
			Major:   "1",
			Minor:   "23",
		},
		ClientId: applicationConfig.ZoneId,
		Log:      loggers["Helm"],
	})
	failIfError(err, setupLog, "unable to create helm client")

	clock := clock.NewClock()
	resourceExcludes := controllerConfig.Cache.ExcludesSet()
	cacheSettings := cache.Settings{
		ResourcesFilter: ResourceFilterFunc(func(group, kind, cluster string) bool {
			key := fmt.Sprintf("%s/%s", group, kind)
			return resourceExcludes[key]
		}),
	}

	clusterCache := cache.NewClusterCache(config,
		cache.SetLogr(loggers["ClusterCache"]),
		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
			managedByMark := un.GetLabels()["dcp.hiro.io/managed-by"]
			info = &types.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
			cacheManifest = managedByMark != ""
			return
		}),
		cache.SetSettings(cacheSettings),
	)
	gitOpsEngine := engine.NewEngine(config, clusterCache, engine.WithLogr(loggers["GitOpsEngine"]))
	stopFunc, err := gitOpsEngine.Run()
	failIfError(err, setupLog, "unable to start gitops engine")

	charts := sync.NewCharts(context.Background(), helmClient, &sync.ChartsOptions{
		SyncPeriod: controllerConfig.Runtime.ChartVersionPollInterval,
	}, loggers["SyncManager"])

	go charts.RunSynchronization()

	applications := sync.NewApplications(
		kubeClient,
		helmClient,
		charts,
		clusterCache,
		clock,
		&applicationConfig,
		gitOpsEngine,
		loggers["SyncManager"],
	)

	jobContext := job.NewAsyncJobContext(helmClient, kubeClient, context.Background(), applications)
	jobs := job.NewJobs(jobContext)
	events := events.NewEvents(mgr.GetEventRecorderFor("Controller"))
	jobFactory := job.NewAsyncJobFactory(&applicationConfig, clock, loggers["Jobs"], &events)
	reconciler := reconciler.NewReconciler(jobs, jobFactory)

	if err = (&controller.AnyApplicationReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Config:       &applicationConfig,
		Applications: applications,
		Jobs:         jobs,
		Reconciler:   reconciler,
		Recorder:     mgr.GetEventRecorderFor("Controller"),
		Log:          loggers["Controller"],
		Events:       &events,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AnyApplication")
		os.Exit(1)
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = webhookdcpv1.SetupAnyApplicationWebhookWithManager(mgr, loggers["Controller"], applicationConfig.ZoneId); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AnyApplication")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		failIfError(mgr.Add(metricsCertWatcher), setupLog, "unable to add metrics certificate watcher to manager")
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		failIfError(mgr.Add(webhookCertWatcher), setupLog, "unable to add webhook certificate watcher to manager")
	}

	failIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), setupLog, "unable to set up health check")
	failIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), setupLog, "unable to set up ready check")
	setupLog.Info("starting Application API Server")

	logFetcher := errorctx.NewRealLogFetcher(clientset)
	applicationReports := errorctx.NewApplicationReports(clusterCache, logFetcher)

	options := httpapi.ApplicationApiOptions{Address: controllerConfig.Api.BindAddress}
	applicationSpecs := resources.NewApplicationSpecs(applications, kubeClient, loggers["API"])
	httpServer := httpapi.NewHttpServer(options, applicationReports, applicationSpecs, &applications, kubeClient)

	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatalf("Http Server start error: %v", err)
		}
	}()

	setupLog.Info("starting manager")
	failIfError(mgr.Start(ctrl.SetupSignalHandler()), setupLog, "problem running manager")
	stopFunc()
}

func generateAndPatchWebhookCertificates(
	clientset *kubernetes.Clientset,
	setupLog logr.Logger,
	webhookCertPath string,
	webhookCertName string,
	webhookCertKey string,
	webhookServiceName string,
) {

	setupLog.Info("No webhook-cert-path provided. Initializing auto-generated self-signed certificates...")

	// Setup a temporary directory to save the certificates for the file watcher
	err := os.MkdirAll(webhookCertPath, 0755)
	failIfError(err, setupLog, "failed to create directory for auto-generated certificates")

	// Deduce parameters or use reasonable defaults (update these to match your actual service name/namespace)
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default" // fallback default
	}
	webhookConfigName := "anyapplication-webhook-configuration"

	certFilePath := filepath.Join(webhookCertPath, webhookCertName)
	certKeyFilePath := filepath.Join(webhookCertPath, webhookCertKey)
	caFilePath := filepath.Join(webhookCertPath, "ca.crt")
	var certificateBundle []byte

	if !fileExists(caFilePath) || !fileExists(certFilePath) || !fileExists(certKeyFilePath) {
		// Generate certs in memory
		caBundle, crt, key, err := generateSelfSignedCert(webhookServiceName, namespace)
		failIfError(err, setupLog, "failed to generate self-signed certificates")
		certificateBundle = caBundle

		// Write to temp filesystem so Kubebuilder's certwatcher can process them normally
		err = os.WriteFile(certFilePath, crt, 0600)
		failIfError(err, setupLog, "failed to write generated certificate to disk")
		setupLog.Info("webhook certificate is saved", "path", certFilePath)

		err = os.WriteFile(certKeyFilePath, key, 0600)
		failIfError(err, setupLog, "failed to write generated key to disk")
		setupLog.Info("webhook certificate key is saved", "path", certKeyFilePath)

		err = os.WriteFile(caFilePath, caBundle, 0600)
		failIfError(err, setupLog, "failed to write CA bundle to disk")
		setupLog.Info("webhook ca bundle is saved", "path", caFilePath)
	} else {
		setupLog.Info("loading existing certificate", "path", caFilePath)
		certificateBundle, err = os.ReadFile(caFilePath)
		failIfError(err, setupLog, "failed to load existing certificate")
	}

	// Patch Validating and Mutating configurations on the cluster api server
	err = patchWebhookCABundle(clientset, setupLog, webhookConfigName, certificateBundle)
	failIfError(err, setupLog, "failed to patch admission webhooks caBundle")
	setupLog.Info("Successfully auto-generated certs and patched webhook caBundles directly.")
}

// Helper function to dynamically generate a self-signed root CA and a serving certificate valid for Kubernetes DNS
func generateSelfSignedCert(serviceName, namespace string) (caBundle, certPEM, keyPEM []byte, err error) {
	// 1. Setup Root CA
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2026),
		Subject: pkix.Name{
			Organization: []string{"hiro.io"},
			CommonName:   "anyapplication-operator-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years validity
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	caPEMBytes := new(bytes.Buffer)
	_ = pem.Encode(caPEMBytes, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})

	// 2. Setup Server Certificate targeted for the specific Kubernetes Service address
	dnsNames := []string{
		serviceName,
		"host.docker.internal",
		"127.0.0.1",
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
	}

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2027),
		Subject: pkix.Name{
			Organization: []string{"hiro.io"},
			CommonName:   fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		},
		DNSNames:    dnsNames,
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // 1 year validity
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEMBytes := new(bytes.Buffer)
	_ = pem.Encode(certPEMBytes, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	keyPEMBytes := new(bytes.Buffer)
	_ = pem.Encode(keyPEMBytes, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)})

	return caPEMBytes.Bytes(), certPEMBytes.Bytes(), keyPEMBytes.Bytes(), nil
}

// Helper function to programmatically update the caBundle field on both Mutating and Validating configurations
func patchWebhookCABundle(clientset *kubernetes.Clientset, setupLog logr.Logger, webhookConfigName string, caBundle []byte) error {
	ctx := context.Background()

	// 1. Attempt to patch ValidatingWebhookConfiguration
	vWebhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhookConfigName, metav1.GetOptions{})
	if err == nil {
		for i := range vWebhookConfig.Webhooks {
			vWebhookConfig.Webhooks[i].ClientConfig.CABundle = caBundle
		}
		_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, vWebhookConfig, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed upgrading validating webhook configuration: %w", err)
		}
		setupLog.Info("Patched ValidatingWebhookConfiguration caBundle.", "name", webhookConfigName)

	}

	return nil
}

func failIfError(err error, log logr.Logger, msg string) {
	if err != nil {
		log.Error(err, msg)
		os.Exit(1)
	}
}

func buildLogger(level zapcore.Level) logr.Logger {
	enabler := LevelEnablerFunc{minLevel: level}
	opts := zap.Options{
		Development: true,
		Level:       enabler,
	}
	return zap.New(zap.UseFlagOptions(&opts))
}

type LevelEnablerFunc struct {
	minLevel zapcore.Level
}

// Enabled returns true if the level is greater than or equal to minLevel
func (l LevelEnablerFunc) Enabled(level zapcore.Level) bool {
	return level >= l.minLevel
}

type ResourceFilterFunc func(group, kind, cluster string) bool

func (f ResourceFilterFunc) IsExcludedResource(group, kind, cluster string) bool {
	return f(group, kind, cluster)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // File exists
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // File does not exist
	}
	// File might exist but has permission issues or other errors
	return false
}
