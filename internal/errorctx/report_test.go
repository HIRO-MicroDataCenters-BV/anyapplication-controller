// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package errorctx

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/fixture"
	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/httpapi/api"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("K8sReportService", func() {
	var (
		fakeClient  *k8sfake.Clientset
		reporter    api.ApplicationReports
		pod         corev1.Pod
		scheme      *runtime.Scheme
		updateFuncs []cache.UpdateSettingsFunc
		logOutput   = "fake logs"
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
		updateFuncs = []cache.UpdateSettingsFunc{
			cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				info = &ctrltypes.ResourceInfo{ManagedByMark: un.GetLabels()["dcp.hiro.io/managed-by"]}
				cacheManifest = true
				return
			}),
		}
		clusterCache, _ := fixture.NewTestClusterCacheWithOptions(updateFuncs, &pod)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		logFetcher := NewRealLogFetcher(fakeClient)
		reporter = NewApplicationReports(clusterCache, logFetcher)
	})

	It("should generate a pod report with logs and events", func() {

		report, err := reporter.Fetch(context.TODO(), "instanceId", "default")
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

	It("should generate a workload report for Deployments, ReplicaSets, StatefulSets, and DaemonSets", func() {
		// Create Deployment
		deployment := fixture.NewDeployment("test-deploy", "default", 3)
		deployment.Labels = map[string]string{
			"dcp.hiro.io/instance-id": "instanceId",
			"dcp.hiro.io/managed-by":  "dcp",
		}
		deployment.Status.AvailableReplicas = 2

		// Create ReplicaSet (orphaned)
		replicaSet := fixture.NewReplicaSet("test-rs", "default", 2)
		replicaSet.Labels = map[string]string{
			"dcp.hiro.io/instance-id": "instanceId",
			"dcp.hiro.io/managed-by":  "dcp",
		}
		replicaSet.Status.ReadyReplicas = 1
		replicaSet.OwnerReferences = nil // orphaned

		// Create StatefulSet
		statefulSet := fixture.NewStatefulSet("test-sts", "default", 2)
		statefulSet.Labels = map[string]string{
			"dcp.hiro.io/instance-id": "instanceId",
			"dcp.hiro.io/managed-by":  "dcp",
		}
		statefulSet.Status.ReadyReplicas = 2

		// Create DaemonSet
		daemonSet := fixture.NewDaemonSet("test-ds", "default")
		daemonSet.Labels = map[string]string{
			"dcp.hiro.io/instance-id": "instanceId",
			"dcp.hiro.io/managed-by":  "dcp",
		}
		daemonSet.Status.DesiredNumberScheduled = 2
		daemonSet.Status.NumberReady = 1

		// Add resources to cluster cache
		clusterCache, _ := fixture.NewTestClusterCacheWithOptions(updateFuncs, deployment, replicaSet, statefulSet, daemonSet)
		if err := clusterCache.EnsureSynced(); err != nil {
			Fail("Failed to sync cluster cache: " + err.Error())
		}

		logFetcher := NewRealLogFetcher(fakeClient)
		reporter := NewApplicationReports(clusterCache, logFetcher)

		report, err := reporter.Fetch(context.TODO(), "instanceId", "default")
		Expect(err).NotTo(HaveOccurred())
		Expect(report.Workloads).NotTo(BeEmpty())

		// Deployment
		deployStatus := api.WorkloadStatus{}
		for _, w := range report.Workloads {
			if w.Kind == "Deployment" && w.Name == "test-deploy" {
				deployStatus = w
				break
			}
		}
		Expect(deployStatus.Name).To(Equal("test-deploy"))
		Expect(deployStatus.Desired).To(Equal(int32(3)))
		Expect(deployStatus.Available).To(Equal(int32(2)))
		Expect(deployStatus.Ready).To(BeFalse())
		Expect(deployStatus.Message).To(ContainSubstring("replicas unavailable"))

		// ReplicaSet
		rsStatus := api.WorkloadStatus{}
		for _, w := range report.Workloads {
			if w.Kind == "ReplicaSet" && w.Name == "test-rs" {
				rsStatus = w
				break
			}
		}
		Expect(rsStatus.Name).To(Equal("test-rs"))
		Expect(rsStatus.Desired).To(Equal(int32(2)))
		Expect(rsStatus.Available).To(Equal(int32(1)))
		Expect(rsStatus.Ready).To(BeFalse())
		Expect(rsStatus.Message).To(ContainSubstring("orphaned replicaset not fully ready"))

		// StatefulSet
		stsStatus := api.WorkloadStatus{}
		for _, w := range report.Workloads {
			if w.Kind == "StatefulSet" && w.Name == "test-sts" {
				stsStatus = w
				break
			}
		}
		Expect(stsStatus.Name).To(Equal("test-sts"))
		Expect(stsStatus.Desired).To(Equal(int32(2)))
		Expect(stsStatus.Available).To(Equal(int32(2)))
		Expect(stsStatus.Ready).To(BeTrue())
		Expect(stsStatus.Message).To(Equal("Healthy"))

		// DaemonSet
		dsStatus := api.WorkloadStatus{}
		for _, w := range report.Workloads {
			if w.Kind == "DaemonSet" && w.Name == "test-ds" {
				dsStatus = w
				break
			}
		}
		Expect(dsStatus.Name).To(Equal("test-ds"))
		Expect(dsStatus.Desired).To(Equal(int32(2)))
		Expect(dsStatus.Available).To(Equal(int32(1)))
		Expect(dsStatus.Ready).To(BeFalse())
		Expect(dsStatus.Message).To(ContainSubstring("daemon pods unavailable"))
	})
})
