package sync

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetAggregatedStatus(
	templateResources []*unstructured.Unstructured,
	managedResourcesByKey map[kube.ResourceKey]*unstructured.Unstructured,
	log logr.Logger,
) *health.HealthStatus {
	statusCounts := 0
	code := health.HealthStatusHealthy
	msg := ""

	for _, obj := range templateResources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)

		liveObj := managedResourcesByKey[resourceKey]
		status := health.HealthStatusHealthy
		message := ""
		if liveObj != nil {
			h, err := health.GetResourceHealth(liveObj, nil)
			if err != nil {
				log.Error(err, "GetResourceHealth failed", "Resource", fullName)
				continue
			}
			if h != nil {
				status = h.Status
				message = h.Message
			}
		} else {
			status = health.HealthStatusMissing
		}
		statusCounts += 1
		if health.IsWorse(code, status) {
			code = status
			msg = msg + ". " + message
		}
	}
	if statusCounts == 0 {
		code = health.HealthStatusUnknown
	}

	status := health.HealthStatus{
		Status:  code,
		Message: msg,
	}
	return &status
}
