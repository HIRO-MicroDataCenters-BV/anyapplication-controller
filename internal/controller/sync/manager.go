package sync

import (
	"context"
	"fmt"
	"sync"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/health"
	gitops_sync "github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cockroachdb/errors"
	"github.com/go-logr/logr"
	"github.com/mittwald/go-helm-client/values"
	"github.com/samber/lo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type cachedApp struct {
	resources  []*unstructured.Unstructured
	revision   string
	instanceId string
	namespace  string
}

type syncManager struct {
	helmClient   helm.HelmClient
	kubeClient   client.Client
	clusterCache cache.ClusterCache
	appCache     sync.Map
	clock        clock.Clock
	config       *config.ApplicationRuntimeConfig
	gitOpsEngine engine.GitOpsEngine
	log          logr.Logger
}

func NewSyncManager(
	kubeClient client.Client,
	helmClient helm.HelmClient,
	clusterCache cache.ClusterCache,
	clock clock.Clock,
	config *config.ApplicationRuntimeConfig,
	gitOpsEngine engine.GitOpsEngine,

) types.SyncManager {
	log := logf.Log.WithName("SyncManager")

	return &syncManager{
		kubeClient:   kubeClient,
		helmClient:   helmClient,
		clusterCache: clusterCache,
		appCache:     sync.Map{},
		clock:        clock,
		config:       config,
		gitOpsEngine: gitOpsEngine,
		log:          log,
	}
}

func (m *syncManager) Sync(ctx context.Context, application *v1.AnyApplication) (*types.SyncResult, error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return types.NewSyncResult(), err
	}
	return m.sync(ctx, app)
}

func (m *syncManager) getOrRenderApp(application *v1.AnyApplication) (*cachedApp, error) {
	appKey := m.getCacheKey(application)
	existing, found := m.appCache.Load(appKey)
	if !found {
		app, err := m.render(application)
		if err != nil {
			return nil, errors.Wrap(err, "Fail to render application")
		}
		m.appCache.Store(appKey, app)
		existing = app
	}
	cached, ok := existing.(*cachedApp)
	if !ok {
		return nil, errors.New("type assertion to *cachedApp failed")
	}
	return cached, nil
}

func (m *syncManager) render(application *v1.AnyApplication) (*cachedApp, error) {
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
	app := cachedApp{
		resources:  objs,
		revision:   helmSelector.Version,
		instanceId: instanceId,
		namespace:  helmSelector.Namespace,
	}
	return &app, nil
}

func (m *syncManager) sync(ctx context.Context, app *cachedApp) (*types.SyncResult, error) {

	syncResult := types.NewSyncResult()
	prune := true

	resourceSyncResults, err := m.gitOpsEngine.Sync(ctx, app.resources, func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		return r.Resource.GetLabels()["dcp.hiro.io/instance-id"] == app.instanceId
	}, app.revision, app.namespace, gitops_sync.WithPrune(prune), gitops_sync.WithLogr(m.log))
	if err != nil {
		m.log.Error(err, "Failed to synchronize cluster state")
		return syncResult, errors.Wrap(err, "Failed to synchronize cluster state")
	}

	m.addAndLogResults(resourceSyncResults, syncResult)

	syncResult.Status = m.getAggregatedStatus(app)
	return syncResult, nil
}

func (m *syncManager) addAndLogResults(resourceSyncResults []common.ResourceSyncResult, syncResult *types.SyncResult) {
	for _, resourceSyncResult := range resourceSyncResults {
		syncResult.AddResult(&resourceSyncResult)

		m.log.V(1).Info("Resource synced",
			"resourceKey", resourceSyncResult.ResourceKey.String(),
			"Version", resourceSyncResult.Version,
			"Status", string(resourceSyncResult.Status),
			"Message", resourceSyncResult.Message,
			"HookType", string(resourceSyncResult.HookType),
			"HookPhase", string(resourceSyncResult.HookPhase),
			"SyncPhase", string(resourceSyncResult.SyncPhase),
		)
	}
}

func (m *syncManager) getAggregatedStatus(app *cachedApp) *health.HealthStatus {
	statusCounts := 0
	code := health.HealthStatusHealthy
	msg := ""

	for _, obj := range app.resources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)

		// Get live object from cache
		live, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool {
			if r.Resource == nil {
				return false
			}
			return r.Resource.GetLabels()["dcp.hiro.io/instance-id"] == app.instanceId
		})
		if err != nil {
			m.log.Error(err, "GetManagedLiveObjs failed", "Resource", fullName)
		}

		liveObj := live[resourceKey]
		if liveObj == nil {
			continue
		}

		h, err := health.GetResourceHealth(liveObj, nil)
		if err != nil {
			m.log.Error(err, "GetResourceHealth failed", "Resource", fullName)
			continue
		}
		if h != nil {
			statusCounts += 1
			if health.IsWorse(code, h.Status) {
				code = h.Status
				msg = msg + "\n" + h.Message
			}
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

func (m *syncManager) Delete(ctx context.Context, application *v1.AnyApplication) (*types.DeleteResult, error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return nil, err
	}

	return m.deleteApp(ctx, application, app)
}

func (m *syncManager) deleteApp(ctx context.Context, application *v1.AnyApplication, app *cachedApp) (*types.DeleteResult, error) {
	syncResult := &types.DeleteResult{}

	syncResult.Total += len(app.resources)

	for _, obj := range app.resources {
		fullName := getFullName(obj)
		m.log.V(1).Info("Deleting resource", "Resource", fullName)

		live, err := m.clusterCache.GetManagedLiveObjs([]*unstructured.Unstructured{obj}, func(r *cache.Resource) bool { return true })
		if err == nil && live != nil {
			err := m.kubeClient.Delete(ctx, obj)

			if err != nil {
				syncResult.DeleteFailed += 1
			} else {
				syncResult.Deleted += 1
				m.log.V(1).Info("Deleted", "Resource", fullName)
			}
		}
	}

	remainingResources := m.findApplicationResources(application)
	syncResult.Total += len(remainingResources)

	for _, obj := range remainingResources {
		fullName := getFullName(obj)
		m.log.V(1).Info("Deleting", "Resource", fullName)

		err := m.kubeClient.Delete(ctx, obj)
		if err != nil {
			syncResult.DeleteFailed += 1
		} else {
			syncResult.Deleted += 1
			m.log.V(1).Info("Deleted", "Resource", fullName)
		}
	}
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
	resources := m.findApplicationResources(application)
	localApplication, err := local.NewFromUnstructured(resources, m.config)
	if err != nil {
		return nil, errors.Wrap(err, "Fail to create local application")
	}
	globalApplication := global.NewFromLocalApplication(localApplication, m.clock, application, m.config)
	return globalApplication, nil
}

func (m *syncManager) findApplicationResources(application *v1.AnyApplication) []*unstructured.Unstructured {
	instanceId := m.getInstanceId(application)
	cachedResources := m.clusterCache.FindResources(application.Spec.Application.HelmSelector.Namespace, func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		labels := r.Resource.GetLabels()
		return labels != nil && labels["dcp.hiro.io/instance-id"] == instanceId
	})
	resources := lo.Values(cachedResources)
	return lo.Map(resources, func(r *cache.Resource, index int) *unstructured.Unstructured { return r.Resource })
}

func getFullName(obj *unstructured.Unstructured) string {
	gvk := obj.GroupVersionKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()
	return fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)
}
