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

package controller

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/textlogger"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/job"
	recon "hiro.io/anyapplication/internal/controller/reconciler"
	"hiro.io/anyapplication/internal/controller/sync"
	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
)

var _ = Describe("AnyApplication Controller", func() {
	var (
		runtimeConfig config.ApplicationRuntimeConfig
		jobs          ctrltypes.AsyncJobs
		syncManager   ctrltypes.SyncManager
		reconciler    recon.Reconciler
		stopFunc      engine.StopFunc
		fakeEvents    events.Events
	)

	Context("When reconciling a resource", func() {

		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		anyapplication := &dcpv1.AnyApplication{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AnyApplication")

			runtimeConfig = config.ApplicationRuntimeConfig{
				ZoneId:                        "zone",
				PollOperationalStatusInterval: time.Duration(60000),
			}

			helmClient, err := helm.NewHelmClient(&helm.HelmClientOptions{
				RestConfig:  cfg,
				Debug:       false,
				Linting:     true,
				KubeVersion: &chartutil.DefaultCapabilities.KubeVersion,
				UpgradeCRDs: true,
			})
			if err != nil {
				panic("error " + err.Error())
			}

			clock := clock.NewClock()
			fakeEvents = events.NewFakeEvents()
			log := textlogger.NewLogger(textlogger.NewConfig())
			clusterCache := cache.NewClusterCache(cfg,
				cache.SetLogr(log),
				cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {

					managedByMark := un.GetLabels()["dcp.hiro.io/managed-by"]
					info = &ctrltypes.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
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

			syncManager = sync.NewSyncManager(k8sClient, helmClient, clusterCache, clock, &runtimeConfig, gitOpsEngine, logf.Log)
			jobContext := job.NewAsyncJobContext(helmClient, k8sClient, ctx, syncManager)
			jobs = job.NewJobs(jobContext)
			jobFactory := job.NewAsyncJobFactory(&runtimeConfig, clock, logf.Log, &fakeEvents)

			reconciler = recon.NewReconciler(jobs, jobFactory)

			err = k8sClient.Get(ctx, typeNamespacedName, anyapplication)
			if err != nil && errors.IsNotFound(err) {
				resource := &dcpv1.AnyApplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: dcpv1.AnyApplicationSpec{
						Application: dcpv1.ApplicationMatcherSpec{
							HelmSelector: &dcpv1.HelmSelectorSpec{
								Repository: "https://helm.nginx.com/stable",
								Chart:      "nginx-ingress",
								Version:    "2.0.1",
								Namespace:  "default",
							},
						},
						Zones: 1,
						PlacementStrategy: dcpv1.PlacementStrategySpec{
							Strategy: dcpv1.PlacementStrategyLocal,
						},
						RecoverStrategy: dcpv1.RecoverStrategySpec{
							Tolerance:  1,
							MaxRetries: 3,
						},
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &dcpv1.AnyApplication{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AnyApplication")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			stopFunc()
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AnyApplicationReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				Config:      &runtimeConfig,
				SyncManager: syncManager,
				Jobs:        jobs,
				Reconciler:  reconciler,
				Log:         logf.Log.WithName("controllers").WithName("AnyApplication"),
				Recorder:    record.NewFakeRecorder(100),
				Events:      &fakeEvents,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
