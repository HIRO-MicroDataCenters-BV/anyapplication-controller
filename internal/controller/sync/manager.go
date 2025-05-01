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
	"github.com/samber/mo"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	clock        clock.Clock
}

func NewSyncManager(
	kubeClient client.Client,
	helmClient helm.HelmClient,
	clusterCache cache.ClusterCache,
	clock clock.Clock,
) types.SyncManager {
	return &syncManager{
		kubeClient:   kubeClient,
		helmClient:   helmClient,
		clusterCache: clusterCache,
		appCache:     sync.Map{},
		clock:        clock,
	}
}

func (m *syncManager) InvalidateCache() error {
	m.clusterCache.Invalidate()
	return nil
}

func (m *syncManager) Sync(ctx context.Context, application *v1.AnyApplication) (types.SyncResult, error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return types.SyncResult{}, err
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

func (m *syncManager) sync(ctx context.Context, app *cachedApp) (types.SyncResult, error) {
	code := health.HealthStatusHealthy
	msg := ""

	syncResult := types.SyncResult{}
	diffConfig, err := m.newDiffConfig()
	if err != nil {
		return syncResult, errors.Wrap(err, "Failed to build diff Config")
	}
	for _, obj := range app.resources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)
		syncResult.Total += 1

		// Get live object from cache
		live, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool { return true })
		created := false
		if err != nil || live[resourceKey] == nil {
			if err := m.kubeClient.Create(ctx, obj); err != nil {
				fmt.Printf("%s Create failed: %v\n", fullName, err)
				syncResult.CreateFailed += 1
			} else {
				fmt.Printf("%s Created successfully. \n", fullName)
				syncResult.Created += 1
				created = true
			}
		}
		liveObj := live[resourceKey]

		if !created {
			// Diff live vs desired
			result, err := argodiff.StateDiff(liveObj, obj, diffConfig)
			if err != nil {
				fmt.Printf("%s Diff failed: %v\n", fullName, err)
				continue
			}

			if result.Modified {
				fmt.Printf("%s Out of sync. Updating...\n", fullName)
				if liveObj != nil {
					obj.SetResourceVersion(liveObj.GetResourceVersion())
				}

				if err := m.kubeClient.Update(ctx, obj); err != nil {
					syncResult.UpdateFailed += 1
					fmt.Printf("%s Update failed: %v\n", fullName, err)
				} else {
					syncResult.Updated += 1
					fmt.Printf("%s Updated successfully.\n", fullName)
				}
			} else {
				fmt.Printf("%s Already in sync.\n", fullName)
			}
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
	syncResult.Status = &status
	return syncResult, nil
}

func (m *syncManager) newDiffConfig() (argodiff.DiffConfig, error) {
	ignoreNormalizerOpts := normalizers.IgnoreNormalizerOpts{}
	ignoreAggregatedRoles := true

	return argodiff.NewDiffConfigBuilder().
		WithNoCache().
		WithDiffSettings(nil, nil, ignoreAggregatedRoles, ignoreNormalizerOpts).
		Build()
}

func (m *syncManager) Delete(ctx context.Context, application *v1.AnyApplication) (types.SyncResult, error) {
	instanceId := m.getInstanceId(application)
	syncResult := types.SyncResult{}
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return syncResult, err
	}
	syncResult.Total += len(app.resources)

	for _, obj := range app.resources {
		fullName := getFullName(obj)
		fmt.Printf("Deleting: %s\n", fullName)

		live, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool { return true })
		if err == nil && live != nil {
			err := m.kubeClient.Delete(ctx, obj)

			if err != nil {
				syncResult.DeleteFailed += 1
			} else {
				syncResult.Deleted += 1
				fmt.Printf("Deleted: %s\n", fullName)
			}
		}
	}

	remainingResources := m.clusterCache.FindResources(application.Spec.Application.HelmSelector.Namespace, func(r *cache.Resource) bool {
		labels := r.Resource.GetLabels()
		return labels["dcp.hiro.io/instance-id"] == instanceId
	})

	syncResult.Total += len(remainingResources)

	for _, resource := range remainingResources {
		obj := resource.Resource
		fullName := getFullName(obj)
		fmt.Printf("Deleting: %s\n", fullName)

		err := m.kubeClient.Delete(ctx, obj)
		if err != nil {
			syncResult.DeleteFailed += 1
		} else {
			syncResult.Deleted += 1
			fmt.Printf("Deleted: %s\n", fullName)
		}
	}

	status := health.HealthStatus{
		Status:  health.HealthStatusMissing,
		Message: "",
	}
	syncResult.Status = &status
	return syncResult, nil
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

func (m *syncManager) LoadApplication(application *v1.AnyApplication) (types.GlobalApplication, error) {
	// local.NewLocalApplicationFromTemplate()
	global.NewFromLocalApplication(mo.None[local.LocalApplication](), m.clock, application, nil)
	return nil, nil
}

func getFullName(obj *unstructured.Unstructured) string {
	gvk := obj.GroupVersionKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()
	return fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)
}
