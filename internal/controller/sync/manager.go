package sync

import (
	"context"
	"fmt"
	"sync"

	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cockroachdb/errors"
	"github.com/mittwald/go-helm-client/values"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cachedApp struct {
	resources []*unstructured.Unstructured
}

type syncManager struct {
	helmClient   helm.HelmClient
	kubeClient   client.Client
	clusterCache cache.ClusterCache
	appCache     sync.Map
}

func NewSyncManager(kubeClient client.Client, helmClient helm.HelmClient, config *rest.Config) SyncManager {
	clusterCache := cache.NewClusterCache(config)
	return &syncManager{
		kubeClient:   kubeClient,
		helmClient:   helmClient,
		clusterCache: clusterCache,
		appCache:     sync.Map{},
	}
}

func (m *syncManager) InvalidateCache() error {
	m.clusterCache.Invalidate()
	return nil
}

func (m *syncManager) Sync(ctx context.Context, application *v1.AnyApplication) (*health.HealthStatus, error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return nil, err
	}

	return m.sync(ctx, app)
}

func (m *syncManager) getOrRenderApp(application *v1.AnyApplication) (*cachedApp, error) {
	appKey := m.getCacheKey(application)
	app, found := m.appCache.Load(appKey)
	if !found {
		resources, err := m.render(application)
		if err != nil {
			return nil, errors.Wrap(err, "Fail to render application")
		}
		app = cachedApp{resources}
		m.appCache.Store(appKey, app)
	}
	cachedApp, ok := app.(cachedApp)
	if !ok {
		return nil, errors.New("failed to assert app as cachedApp")
	}
	return &cachedApp, nil
}

func (m *syncManager) render(application *v1.AnyApplication) ([]*unstructured.Unstructured, error) {
	releaseName := application.Name
	helmSelector := application.Spec.Application.HelmSelector
	instanceId := m.getInstanceId(application)

	labels := map[string]string{
		"dcp.hiro.io/managed-by":  "dcp",
		"dcp.hiro.io/instance-id": instanceId,
	}

	values := values.Options{}
	template, err := m.helmClient.Template(&helm.TemplateArgs{
		ReleaseName:   releaseName,
		RepoUrl:       helmSelector.Repository,
		ChartName:     helmSelector.Chart,
		Namespace:     helmSelector.Namespace,
		Version:       helmSelector.Version,
		ValuesOptions: values,
		Labels:        labels,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Helm template failure")
	}

	objs, err := kube.SplitYAML([]byte(template))
	if err != nil {
		return nil, errors.Wrap(err, "Fail to split YAML")
	}

	return objs, nil
}

func (m *syncManager) sync(ctx context.Context, app *cachedApp) (*health.HealthStatus, error) {
	code := health.HealthStatusHealthy
	msg := ""

	ignoreNormalizerOpts := normalizers.IgnoreNormalizerOpts{}
	ignoreAggregatedRoles := true

	diffConfig, err := argodiff.NewDiffConfigBuilder().
		WithNoCache().
		WithDiffSettings(nil, nil, ignoreAggregatedRoles, ignoreNormalizerOpts).
		Build()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build diff Config")

	}

	for _, obj := range app.resources {
		gvk := obj.GroupVersionKind()
		name := obj.GetName()
		namespace := obj.GetNamespace()
		fullName := fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)

		resourceKey := kube.GetResourceKey(obj)
		// Get live object from cache
		live, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool { return true })
		if err != nil || live[resourceKey] == nil {
			if err := m.kubeClient.Create(ctx, obj); err != nil {
				fmt.Printf("%s Create failed: %v\n", fullName, err)
			} else {
				fmt.Printf("%s Created successfully. \n", fullName)
			}
		}
		liveObj := live[resourceKey]

		// Diff live vs desired
		result, err := argodiff.StateDiff(liveObj, obj, diffConfig)
		// result, err := diff.Diff(liveObj, obj)
		if err != nil {
			fmt.Printf("%s Diff failed: %v\n", fullName, err)
			continue
		}

		if result.Modified {
			fmt.Printf("%s Out of sync. Updating...\n", fullName)
			obj.SetResourceVersion(liveObj.GetResourceVersion())
			if err := m.kubeClient.Update(ctx, obj); err != nil {
				fmt.Printf("%s Update failed: %v\n", fullName, err)
			} else {
				fmt.Printf("%s Updated successfully.\n", fullName)
			}
		} else {
			fmt.Printf("%s Already in sync.\n", fullName)
		}

		h, err := health.GetResourceHealth(obj, nil)

		if err != nil {
			fmt.Printf("%s check health failed: %v\n", fullName, err)
			continue
		}
		if h != nil {
			if health.IsWorse(code, h.Status) {
				code = h.Status
				msg = msg + "\n" + h.Message
			}
		}
	}

	status := health.HealthStatus{
		Status:  code,
		Message: msg,
	}

	return &status, nil
}

func (m *syncManager) Delete(ctx context.Context, application *v1.AnyApplication) error {
	releaseName := application.Name
	helmSelector := application.Spec.Application.HelmSelector
	instanceId := helmSelector.Chart + "-" + helmSelector.Version + "-" + releaseName

	app, err := m.getOrRenderApp(application)
	if err != nil {
		return err
	}

	for _, obj := range app.resources {
		gvk := obj.GroupVersionKind()
		name := obj.GetName()
		namespace := obj.GetNamespace()
		fullName := fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)
		fmt.Printf("Deleting: %s\n", fullName)

		// resourceKey := kube.GetResourceKey(obj)
		_, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool { return true })
		if err == nil {
			err := m.kubeClient.Delete(ctx, obj)
			if err != nil {
				return errors.Wrap(err, "Fail to delete object "+fullName)
			}
		}
	}
	remainingResources := m.clusterCache.FindResources(application.Spec.Application.HelmSelector.Namespace, func(r *cache.Resource) bool {
		labels := r.Resource.GetLabels()
		return labels["dcp.hiro.io/instance-id"] == instanceId
	})

	for _, resource := range remainingResources {
		obj := resource.Resource
		gvk := obj.GroupVersionKind()
		name := obj.GetName()
		namespace := obj.GetNamespace()

		fullName := fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)
		fmt.Printf("Deleting: %s\n", fullName)

		err := m.kubeClient.Delete(ctx, obj)
		if err != nil {
			return errors.Wrap(err, "Fail to delete object "+fullName)
		}
	}

	return nil
}

func (m *syncManager) getCacheKey(application *v1.AnyApplication) string {
	version := application.Spec.Application.HelmSelector.Version
	return fmt.Sprintf("%s-%s-%s", application.Name, application.Namespace, version)
}

func (m *syncManager) getInstanceId(application *v1.AnyApplication) string {
	releaseName := application.Name
	helmSelector := application.Spec.Application.HelmSelector
	return fmt.Sprintf("%s-%s-%s", helmSelector.Chart, helmSelector.Version, releaseName)
}
