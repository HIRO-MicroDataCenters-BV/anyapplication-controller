package local

import (
	"context"
	"encoding/json"
	"log"

	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationBundle struct {
	Services     *unstructured.UnstructuredList
	Deployments  *unstructured.UnstructuredList
	StatefulSets *unstructured.UnstructuredList
	Jobs         *unstructured.UnstructuredList
	DaemonSets   *unstructured.UnstructuredList
	Secrets      *unstructured.UnstructuredList
	ConfigMaps   *unstructured.UnstructuredList
}

// Also CronJob, Ingress, NetworkPolicy, PVC, ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding

func LoadApplicationBundle(ctx context.Context, client client.Client, applicationSpec *v1.ApplicationMatcherSpec) (ApplicationBundle, error) {
	serviceList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "",
		Kind:    "ServiceList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading Service:", err)
		return ApplicationBundle{}, err
	}

	deploymentsList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DeploymentList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading Deployment:", err)
		return ApplicationBundle{}, err
	}

	statefulsetList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "StatefulSetList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading StatefulSet:", err)
		return ApplicationBundle{}, err
	}

	jobsList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "batch",
		Kind:    "JobList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading Jobs:", err)
		return ApplicationBundle{}, err
	}

	daemonsetList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DaemonSetList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading DaemonSets:", err)
		return ApplicationBundle{}, err
	}

	secretList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "",
		Kind:    "SecretList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading Secrets:", err)
		return ApplicationBundle{}, err
	}

	configMapList, err := loadResource(ctx, client, applicationSpec, schema.GroupVersionKind{
		Group:   "",
		Kind:    "ConfigMapList",
		Version: "v1",
	})
	if err != nil {
		log.Println("Error loading ConfigMap:", err)
		return ApplicationBundle{}, err
	}

	return ApplicationBundle{
		Deployments:  &deploymentsList,
		StatefulSets: &statefulsetList,
		Jobs:         &jobsList,
		DaemonSets:   &daemonsetList,
		Services:     &serviceList,
		Secrets:      &secretList,
		ConfigMaps:   &configMapList,
	}, nil
}

func loadResource(
	ctx context.Context,
	k8sClient client.Client,
	applicationSpec *v1.ApplicationMatcherSpec,
	kind schema.GroupVersionKind,
) (unstructured.UnstructuredList, error) {
	resources := unstructured.UnstructuredList{}
	resources.SetGroupVersionKind(kind)
	opts := []client.ListOption{
		// client.InNamespace(namespace),
		client.MatchingLabels(applicationSpec.ResourceSelector),
	}
	err := k8sClient.List(ctx, &resources, opts...)

	return resources, err
}

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

func (bundle *ApplicationBundle) CleanResources() ApplicationBundle {
	Deployments := bundle.Deployments.DeepCopy()
	Services := bundle.Services.DeepCopy()
	StatefulSets := bundle.StatefulSets.DeepCopy()
	Jobs := bundle.Jobs.DeepCopy()
	DaemonSets := bundle.DaemonSets.DeepCopy()
	Secrets := bundle.Secrets.DeepCopy()
	ConfigMaps := bundle.ConfigMaps.DeepCopy()

	Deployments.Items = Map(Deployments.Items, cleanResource)
	Services.Items = Map(Services.Items, cleanResource)
	StatefulSets.Items = Map(StatefulSets.Items, cleanResource)
	Jobs.Items = Map(Jobs.Items, cleanResource)
	DaemonSets.Items = Map(DaemonSets.Items, cleanResource)
	Secrets.Items = Map(Secrets.Items, cleanResource)
	ConfigMaps.Items = Map(ConfigMaps.Items, cleanResource)

	cleanBundle := ApplicationBundle{
		Deployments:  Deployments,
		Services:     Services,
		StatefulSets: StatefulSets,
		Jobs:         Jobs,
		DaemonSets:   DaemonSets,
		Secrets:      Secrets,
		ConfigMaps:   ConfigMaps,
	}

	return cleanBundle
}

func cleanResource(resource unstructured.Unstructured) unstructured.Unstructured {

	log.Default().Printf("kind %s", resource.GetKind())

	switch resource.GetKind() {
	case "Service":
		// Clean Service-specific runtime details
		unstructured.RemoveNestedField(resource.Object, "status")
		unstructured.RemoveNestedField(resource.Object, "spec", "clusterIP")
		unstructured.RemoveNestedField(resource.Object, "spec", "clusterIPs")

	case "Deployment":
		// Clean Deployment-specific runtime details
		unstructured.RemoveNestedField(resource.Object, "status")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "nodeName")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "affinity")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "tolerations")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "hostAliases")
	case "StatefulSet":
		// Clean StatefulSet-specific runtime details
		unstructured.RemoveNestedField(resource.Object, "status")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "nodeName")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "affinity")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "tolerations")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "hostAliases")
	case "DaemonSet":
		// Clean DaemonSet-specific runtime details
		unstructured.RemoveNestedField(resource.Object, "status")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "nodeName")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "affinity")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "tolerations")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "hostAliases")

	case "Job":
		// Clean Job-specific runtime details
		unstructured.RemoveNestedField(resource.Object, "status")
		unstructured.RemoveNestedField(resource.Object, "spec", "template", "spec", "nodeName")

	case "Secret":
	case "ConfigMap":
		// Clean Secret/ConfigMap-specific runtime details
	default:
		// Handle other resources similarly if needed (StatefulSets, ReplicaSets, etc.)
	}

	// Clean metadata (like `status`, `resourceVersion`, etc.)
	unstructured.RemoveNestedField(resource.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(resource.Object, "metadata", "uid")
	unstructured.RemoveNestedField(resource.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(resource.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(resource.Object, "metadata", "generation")
	unstructured.RemoveNestedField(resource.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(resource.Object, "metadata", "deletionTimestamp")
	unstructured.RemoveNestedField(resource.Object, "metadata", "finalizers")
	unstructured.RemoveNestedField(resource.Object, "metadata", "ownerReferences")
	unstructured.RemoveNestedField(resource.Object, "metadata", "clusterName")
	return resource
}

func Map[T any, R any](slice []T, f func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func (bundle *ApplicationBundle) DetermineState() LocalState {
	// deploymentStates := Map(bundle.Deployments.Items, determineDeploymentState)
	// statefulSetStates := Map(bundle.StatefulSets.Items, determineStatefulSetState)
	// daemonSetStates := Map(bundle.DaemonSets.Items, determineDaemonSetState)
	// allStates := append(deploymentStates, statefulSetStates...)
	// allStates = append(allStates, daemonSetStates...)

	// if at least on is failed then Completed with failure
	// if at least one is starting then starting
	// if all are running then running

	// states := lo.Uniq(allStates)

	// result := lo.GroupByMap(groupedStates, func(v []LocalState) (LocalState, int) {
	// 	return v[0], len(v)
	// })

	return NewLocal
}

// func determineDeploymentState(deployment unstructured.Unstructured) LocalState {
// 	// return health.GetResourceHealth(deployment)
// 	return NewLocal
// }

// func determineStatefulSetState(deployment unstructured.Unstructured) LocalState {
// 	// return health.GetResourceHealth(deployment)
// 	return NewLocal
// }

// func determineDaemonSetState(deployment unstructured.Unstructured) LocalState {
// 	// Implement logic to determine the state of the deployment
// 	return NewLocal
// }
