package local

import (
	"encoding/json"
	"log"

	health "github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ApplicationBundle struct {
	resources []*unstructured.Unstructured
	// Services     *unstructured.UnstructuredList
	// Deployments  *unstructured.UnstructuredList
	// StatefulSets *unstructured.UnstructuredList
	// Jobs         *unstructured.UnstructuredList
	// DaemonSets   *unstructured.UnstructuredList
	// Secrets      *unstructured.UnstructuredList
	// ConfigMaps   *unstructured.UnstructuredList
}

// func LoadApplicationBundle(ctx context.Context, client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (ApplicationBundle, error) {
// 	serviceList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "",
// 		Kind:    "ServiceList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading Service:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	deploymentsList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "apps",
// 		Kind:    "DeploymentList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading Deployment:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	statefulsetList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "apps",
// 		Kind:    "StatefulSetList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading StatefulSet:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	jobsList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "batch",
// 		Kind:    "JobList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading Jobs:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	daemonsetList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "apps",
// 		Kind:    "DaemonSetList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading DaemonSets:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	secretList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "",
// 		Kind:    "SecretList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading Secrets:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	configMapList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
// 		Group:   "",
// 		Kind:    "ConfigMapList",
// 		Version: "v1",
// 	})
// 	if err != nil {
// 		log.Println("Error loading ConfigMap:", err)
// 		return ApplicationBundle{}, err
// 	}

// 	return ApplicationBundle{
// 		Deployments:  &deploymentsList,
// 		StatefulSets: &statefulsetList,
// 		Jobs:         &jobsList,
// 		DaemonSets:   &daemonsetList,
// 		Services:     &serviceList,
// 		Secrets:      &secretList,
// 		ConfigMaps:   &configMapList,
// 	}, nil
// }

func LoadApplicationBundle(resources []*unstructured.Unstructured) (ApplicationBundle, error) {
	return ApplicationBundle{
		resources: resources,
	}, nil
}

// func loadResource(
// 	ctx context.Context,
// 	k8sClient client.Client,
// 	applicationSpec *v1.ApplicationMatcherSpec,
// 	kind schema.GroupVersionKind,
// ) (unstructured.UnstructuredList, error) {
// 	resources := unstructured.UnstructuredList{}
// 	resources.SetGroupVersionKind(kind)
// 	opts := []client.ListOption{
// 		// client.InNamespace(namespace),
// 		client.MatchingLabels(*applicationSpec.ResourceSelector),
// 	}
// 	err := k8sClient.List(ctx, &resources, opts...)

// 	return resources, err
// }

func Deserialize(data string) (ApplicationBundle, error) {
	var bundle ApplicationBundle
	err := json.Unmarshal([]byte(data), &bundle)
	if err != nil {
		log.Fatal(err)
	}
	return bundle, nil
}

func (bundle *ApplicationBundle) Serialize() (string, error) {

	jsonData, err := json.Marshal(bundle)
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonData), nil
}

func Map[T any, R any](slice []T, f func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func (bundle *ApplicationBundle) DetermineState() (health.HealthStatusCode, []string, error) {
	resourceStatuses, err := foldLeft(bundle.resources, make([]health.HealthStatus, 0),
		func(acc []health.HealthStatus, item *unstructured.Unstructured) ([]health.HealthStatus, error) {
			status, err := determineResourceState(item)
			if err != nil {
				return acc, err
			}
			if status != nil {
				acc = append(acc, *status)
			}
			return acc, nil
		})
	if err != nil {
		return health.HealthStatusUnknown, nil, err
	}

	healthStatus, _ := foldLeft(resourceStatuses, health.HealthStatusHealthy,
		func(acc health.HealthStatusCode, item health.HealthStatus) (health.HealthStatusCode, error) {
			if health.IsWorse(acc, item.Status) {
				acc = item.Status
			}
			return acc, nil
		},
	)

	messages, _ := foldLeft(resourceStatuses, make([]string, 0),
		func(acc []string, item health.HealthStatus) ([]string, error) {
			if item.Message != "" {
				acc = append(acc, item.Message)
			}
			return acc, nil
		},
	)

	return healthStatus, messages, nil
}

func determineResourceState(resource *unstructured.Unstructured) (*health.HealthStatus, error) {
	status, err := health.GetResourceHealth(resource, nil)
	return status, err
}

func foldLeft[T, R any](arr []T, initial R, fn func(acc R, item T) (R, error)) (R, error) {
	acc := initial
	for _, item := range arr {
		var err error
		acc, err = fn(acc, item)
		if err != nil {
			return acc, err // Return the accumulator and the error if one occurred
		}
	}
	return acc, nil
}
