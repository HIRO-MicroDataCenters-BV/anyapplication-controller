// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package errorctx

import (
	"context"
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/cache"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
	"hiro.io/anyapplication/internal/httpapi/api"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const CONST_HEALTHY = "Healthy"

type applicationReports struct {
	clusterCache cache.ClusterCache
	logFetcher   LogFetcher
}

func NewApplicationReports(clusterCache cache.ClusterCache, logFetcher LogFetcher) api.ApplicationReports {
	return &applicationReports{clusterCache: clusterCache, logFetcher: logFetcher}
}

func (s *applicationReports) Fetch(
	ctx context.Context,
	instanceId string,
	namespace string,
) (*api.ApplicationReport, error) {

	report := &api.ApplicationReport{}
	podInfo, err := s.GetPodInfo(ctx, instanceId, namespace)
	if err != nil {
		return nil, fmt.Errorf("getting pod info: %w", err)
	}
	report.Pods = podInfo

	workloadStatuses, err := s.GetWorkloadStatuses(instanceId)
	if err != nil {
		return nil, fmt.Errorf("getting workload statuses: %w", err)
	}
	report.Workloads = workloadStatuses

	return report, nil
}

func (s *applicationReports) GetPodInfo(
	ctx context.Context,
	instanceId string,
	namespace string,
) ([]api.PodInfo, error) {

	pods := s.getPods(instanceId)
	events, err := s.logFetcher.FetchEvents(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("fetching events: %w", err)
	}
	report := []api.PodInfo{}
	for _, pod := range pods {
		podInfo := api.PodInfo{
			Name:     pod.Name,
			Status:   string(pod.Status.Phase),
			Restarts: 0,
			Events:   []api.PodEvent{},
			Logs:     []api.LogInfo{},
		}

		for _, cs := range pod.Status.ContainerStatuses {
			podInfo.Restarts += cs.RestartCount

			if cs.State.Waiting != nil {
				podInfo.Status = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil {
				podInfo.Status = cs.State.Terminated.Reason
			}

			logStr, logErr := s.logFetcher.FetchLogs(ctx, namespace, pod.Name, cs.Name, cs.RestartCount > 0)
			if logErr != nil {
				logStr = fmt.Sprintf("Error fetching logs: %v", logErr)
			}
			podInfo.Logs = append(podInfo.Logs, api.LogInfo{
				Container: cs.Name,
				Log:       truncate(logStr, 1000),
			})
		}

		for _, e := range events.Items {
			if e.InvolvedObject.Kind == "Pod" && e.InvolvedObject.Name == pod.Name && e.Type == "Warning" {
				podInfo.Events = append(podInfo.Events, api.PodEvent{
					Reason:    e.Reason,
					Message:   e.Message,
					Timestamp: e.FirstTimestamp.String(),
				})
			}
		}

		report = append(report, podInfo)
	}

	return report, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... [truncated]"
}

func (s *applicationReports) getPods(instanceId string) []*corev1.Pod {
	podsUnstructured := s.findResources(instanceId, mapset.NewSet("Pod"))
	return convertTo(podsUnstructured, &corev1.Pod{})

}

func (s *applicationReports) findResources(instanceId string, resourceTypes mapset.Set[string]) []*unstructured.Unstructured {
	cachedResources := s.clusterCache.FindResources("", func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		labels := r.Resource.GetLabels()
		return labels != nil && labels["dcp.hiro.io/instance-id"] == instanceId && resourceTypes.Contains(r.Resource.GetKind())
	})
	resources := lo.Values(cachedResources)
	return lo.Map(resources, func(r *cache.Resource, index int) *unstructured.Unstructured { return r.Resource })
}

func convertTo[T runtime.Object](resources []*unstructured.Unstructured, sample T) []T {
	objects := make([]T, 0, len(resources))
	for _, cachedResource := range resources {
		obj := sample.DeepCopyObject().(T)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(cachedResource.Object, obj)
		if err != nil {
			fmt.Printf("Error converting unstructured to object: %v\n", err)
			continue
		}
		objects = append(objects, obj)
	}
	return objects
}

func (r *applicationReports) GetWorkloadStatuses(instanceId string) ([]api.WorkloadStatus, error) {
	report := make([]api.WorkloadStatus, 0)

	deploymentStatuses := r.getDeployments(instanceId)
	report = append(report, deploymentStatuses...)

	rsStatuses := r.getReplicaSets(instanceId)
	report = append(report, rsStatuses...)

	stsStatuses := r.getStatefulSets(instanceId)
	report = append(report, stsStatuses...)

	dsStatuses := r.getDaemonSets(instanceId)
	report = append(report, dsStatuses...)

	return report, nil
}

// --- Deployment ---
func (s *applicationReports) getDeployments(instanceId string) []api.WorkloadStatus {
	deploymentsUnstructured := s.findResources(instanceId, mapset.NewSet("Deployment"))
	deployments := convertTo(deploymentsUnstructured, &appsv1.Deployment{})

	statuses := make([]api.WorkloadStatus, 0, len(deployments))
	for _, d := range deployments {

		status := api.WorkloadStatus{
			Kind:      "Deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Desired:   *d.Spec.Replicas,
			Available: d.Status.AvailableReplicas,
			Message:   CONST_HEALTHY,
		}

		unavailable := status.Desired - status.Available
		status.Unavailable = unavailable
		status.Ready = unavailable == 0

		if !status.Ready {
			status.Message = fmt.Sprintf("%d of %d replicas unavailable", unavailable, status.Desired)
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func (s *applicationReports) getReplicaSets(instanceId string) []api.WorkloadStatus {
	replicaSetsUnstructured := s.findResources(instanceId, mapset.NewSet("ReplicaSet"))
	replicaSets := convertTo(replicaSetsUnstructured, &appsv1.ReplicaSet{})

	statuses := make([]api.WorkloadStatus, 0, len(replicaSets))
	for _, rs := range replicaSets {
		// Ignore owned ReplicaSets
		if len(rs.OwnerReferences) > 0 {
			continue
		}

		status := api.WorkloadStatus{
			Kind:      "ReplicaSet",
			Name:      rs.Name,
			Namespace: rs.Namespace,
			Desired:   *rs.Spec.Replicas,
			Available: rs.Status.ReadyReplicas,
			Ready:     rs.Status.ReadyReplicas == *rs.Spec.Replicas,
		}
		status.Unavailable = status.Desired - status.Available
		if !status.Ready {
			status.Message = fmt.Sprintf("orphaned replicaset not fully ready (%d/%d)", status.Available, status.Desired)
		} else {
			status.Message = CONST_HEALTHY
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// --- StatefulSet ---
func (s *applicationReports) getStatefulSets(instanceId string) []api.WorkloadStatus {
	statefulSetsUnstructured := s.findResources(instanceId, mapset.NewSet("StatefulSet"))
	statefulSets := convertTo(statefulSetsUnstructured, &appsv1.StatefulSet{})

	statuses := make([]api.WorkloadStatus, 0, len(statefulSets))
	for _, sts := range statefulSets {
		status := api.WorkloadStatus{
			Kind:      "StatefulSet",
			Name:      sts.Name,
			Namespace: sts.Namespace,
			Desired:   *sts.Spec.Replicas,
			Available: sts.Status.ReadyReplicas,
			Ready:     sts.Status.ReadyReplicas == *sts.Spec.Replicas,
		}
		status.Unavailable = status.Desired - status.Available
		if !status.Ready {
			status.Message = fmt.Sprintf("statefulset not fully ready (%d/%d)", status.Available, status.Desired)
		} else {
			status.Message = CONST_HEALTHY
		}
		statuses = append(statuses, status)
	}
	return statuses
}
func (s *applicationReports) getDaemonSets(instanceId string) []api.WorkloadStatus {
	daemonSetsUnstructured := s.findResources(instanceId, mapset.NewSet("DaemonSet"))
	daemonSets := convertTo(daemonSetsUnstructured, &appsv1.DaemonSet{})

	statuses := make([]api.WorkloadStatus, 0, len(daemonSets))
	for _, ds := range daemonSets {
		status := api.WorkloadStatus{
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Namespace: ds.Namespace,
			Desired:   ds.Status.DesiredNumberScheduled,
			Available: ds.Status.NumberReady,
			Ready:     ds.Status.NumberReady == ds.Status.DesiredNumberScheduled,
		}
		status.Unavailable = status.Desired - status.Available
		if !status.Ready {
			status.Message = fmt.Sprintf("%d daemon pods unavailable", status.Unavailable)
		} else {
			status.Message = CONST_HEALTHY
		}
		statuses = append(statuses, status)
	}
	return statuses
}
