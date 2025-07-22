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
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

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
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var configurationFile string
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
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

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
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

	webhookServer := webhook.NewServer(webhook.Options{TLSOpts: webhookTLSOpts})

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

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
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
	// config := ctrl.GetConfigOrDie()
	config, err := configctrl.GetConfigWithContext(applicationConfig.ZoneId)
	failIfError(err, setupLog, "unable to get config")
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
		UpgradeCRDs: true,
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

	applications := sync.NewApplications(
		kubeClient,
		helmClient,
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

	clientset, err := kubernetes.NewForConfig(config)
	failIfError(err, setupLog, "unable to create kubernetes clientset for log fetcher")
	logFetcher := errorctx.NewRealLogFetcher(clientset)
	applicationReports := errorctx.NewApplicationReports(clusterCache, logFetcher)

	options := httpapi.ApplicationApiOptions{Address: controllerConfig.Api.BindAddress}
	httpServer := httpapi.NewHttpServer(options, applicationReports, &applications, kubeClient)

	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatalf("Http Server start error: %v", err)
		}
	}()

	setupLog.Info("starting manager")
	failIfError(mgr.Start(ctrl.SetupSignalHandler()), setupLog, "problem running manager")
	stopFunc()
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
