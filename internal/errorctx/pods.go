package errorctx

import (
	"context"
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/cache"
	dcpv1 "hiro.io/anyapplication/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type K8sReportService struct {
	clusterCache cache.ClusterCache
	logFetcher   LogFetcher
}

func NewK8sReportService(clusterCache cache.ClusterCache, logFetcher LogFetcher) *K8sReportService {
	return &K8sReportService{clusterCache: clusterCache, logFetcher: logFetcher}
}

func (s *K8sReportService) GeneratePodReport(ctx context.Context, instanceId string, namespace string) (*dcpv1.PodReport, error) {

	pods := s.getPods(instanceId)

	events, err := s.logFetcher.FetchEvents(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("fetching events: %w", err)
	}
	fmt.Printf("Found %d pods and %d events in namespace %s\n", len(pods), len(events.Items), namespace)
	report := &dcpv1.PodReport{}

	for _, pod := range pods {
		pInfo := dcpv1.PodInfo{
			Name:     pod.Name,
			Status:   string(pod.Status.Phase),
			Restarts: 0,
			Events:   []dcpv1.PodEvent{},
			Logs:     []dcpv1.LogInfo{},
		}

		for _, cs := range pod.Status.ContainerStatuses {
			pInfo.Restarts += cs.RestartCount

			if cs.State.Waiting != nil {
				pInfo.Status = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil {
				pInfo.Status = cs.State.Terminated.Reason
			}

			logStr, logErr := s.logFetcher.FetchLogs(ctx, namespace, pod.Name, cs.Name, cs.RestartCount > 0)
			if logErr != nil {
				logStr = fmt.Sprintf("Error fetching logs: %v", logErr)
			}
			pInfo.Logs = append(pInfo.Logs, dcpv1.LogInfo{
				Container: cs.Name,
				Log:       truncate(logStr, 1000),
			})
		}

		for _, e := range events.Items {
			if e.InvolvedObject.Kind == "Pod" && e.InvolvedObject.Name == pod.Name && e.Type == "Warning" {
				pInfo.Events = append(pInfo.Events, dcpv1.PodEvent{
					Reason:    e.Reason,
					Message:   e.Message,
					Timestamp: e.FirstTimestamp.String(),
				})
			}
		}

		report.Pods = append(report.Pods, pInfo)
	}

	return report, nil
}

func (s *K8sReportService) getPods(instanceId string) []corev1.Pod {
	cachedResources := s.clusterCache.FindResources("", func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		labels := r.Resource.GetLabels()

		return labels != nil && labels["dcp.hiro.io/instance-id"] == instanceId && r.Resource.GetKind() == "Pod"
	})

	pods := make([]corev1.Pod, 0, len(cachedResources))
	for _, cachedResource := range cachedResources {

		var pod corev1.Pod
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(cachedResource.Resource.Object, &pod)
		if err != nil {
			fmt.Printf("Error converting unstructured to Pod: %v\n", err)
		}
		pods = append(pods, pod)
	}
	return pods
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... [truncated]"
}
