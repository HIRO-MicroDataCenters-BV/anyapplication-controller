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
	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/global"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cachedApp struct {
	application *v1.AnyApplication
	resources   []*unstructured.Unstructured
	revision    string
	instanceId  string
	namespace   string
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
	logger logr.Logger,
) types.SyncManager {
	log := logger.WithName("SyncManager")
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
	helmSelector := application.Spec.Source.HelmSelector
	instanceId := m.GetInstanceId(application)

	labels := map[string]string{
		"dcp.hiro.io/managed-by":  "dcp",
		"dcp.hiro.io/instance-id": instanceId,
	}

	template, err := m.helmClient.Template(&helm.TemplateArgs{
		ReleaseName: releaseName,
		RepoUrl:     helmSelector.Repository,
		ChartName:   helmSelector.Chart,
		Namespace:   helmSelector.Namespace,
		Version:     helmSelector.Version,
		ValuesYaml:  helmSelector.Values,
		Labels:      labels,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Helm template failure")
	}
	objs, err := kube.SplitYAML([]byte(template))
	if err != nil {
		return nil, errors.Wrap(err, "Fail to split YAML")
	}
	app := cachedApp{
		resources:   objs,
		revision:    helmSelector.Version,
		instanceId:  instanceId,
		namespace:   helmSelector.Namespace,
		application: application.DeepCopy(),
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

	localApplicationOpt, err := m.loadLocalApplication(app.application)
	localApplication, exists := localApplicationOpt.Get()
	if err == nil {
		syncResult.ApplicationResourcesPresent = exists
		if exists {
			syncResult.ApplicationResourcesDeployed = localApplication.IsDeployed()
		}
	} else {
		m.log.Error(err, "Failed to load local application")
		syncResult.ApplicationResourcesDeployed = false
		syncResult.ApplicationResourcesPresent = false
	}
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

func (m *syncManager) GetAggregatedStatus(application *v1.AnyApplication) *health.HealthStatus {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		m.log.Error(err, "Failed to get or render application")
	}
	return m.getAggregatedStatus(app)
}

func (m *syncManager) getAggregatedStatus(app *cachedApp) *health.HealthStatus {
	statusCounts := 0
	code := health.HealthStatusHealthy
	msg := ""

	managedResources := m.findAvailableApplicationResources(app.application)
	managedResourcesByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for _, res := range managedResources {
		key := kube.GetResourceKey(res)
		managedResourcesByKey[key] = res
	}

	for _, obj := range app.resources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)

		liveObj := managedResourcesByKey[resourceKey]
		status := health.HealthStatusHealthy
		message := ""
		if liveObj != nil {
			h, err := health.GetResourceHealth(liveObj, nil)
			if err != nil {
				m.log.Error(err, "GetResourceHealth failed", "Resource", fullName)
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

func (m *syncManager) Delete(ctx context.Context, application *v1.AnyApplication) (*types.DeleteResult, error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return nil, err
	}

	return m.deleteApp(ctx, app)
}

func (m *syncManager) deleteApp(ctx context.Context, app *cachedApp) (*types.DeleteResult, error) {
	syncResult := &types.DeleteResult{}

	syncResult.Total += len(app.resources)

	managedResources := m.findAvailableApplicationResources(app.application)
	managedResourcesByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for _, res := range managedResources {
		key := kube.GetResourceKey(res)
		managedResourcesByKey[key] = res
	}
	// Deleting known managed resources
	for _, obj := range app.resources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)
		m.log.V(1).Info("Deleting resource", "Resource", fullName)
		live := managedResourcesByKey[resourceKey]

		if live == nil {
			syncResult.Deleted += 1
		} else {
			err := m.kubeClient.Delete(ctx, obj)
			if err != nil {
				syncResult.DeleteFailed += 1
			} else {
				delete(managedResourcesByKey, resourceKey)
				syncResult.Deleted += 1
				m.log.V(1).Info("Deleted", "Resource", fullName)
			}
		}
	}

	// Deleting remaining managed resources
	for _, obj := range managedResourcesByKey {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)
		m.log.V(1).Info("Deleting", "Resource", fullName)

		err := m.kubeClient.Delete(ctx, obj)

		if err != nil {
			syncResult.DeleteFailed += 1
		} else {
			delete(managedResourcesByKey, resourceKey)
			syncResult.Deleted += 1
			m.log.V(1).Info("Deleted", "Resource", fullName)
		}
	}
	fmt.Printf("managedResourcesByKey %d, deleted %d, delete failures %d\n", len(managedResourcesByKey), syncResult.Deleted, syncResult.DeleteFailed)
	syncResult.ApplicationResourcesPresent = len(managedResourcesByKey) > 0
	return syncResult, nil

}

func (m *syncManager) getCacheKey(application *v1.AnyApplication) string {
	version := application.Spec.Source.HelmSelector.Version
	return fmt.Sprintf("%s-%s-%s", application.Name, application.Namespace, version)
}

func (m *syncManager) GetInstanceId(application *v1.AnyApplication) string {
	releaseName := application.Name
	helmSelector := application.Spec.Source.HelmSelector
	return fmt.Sprintf("%s-%s-%s", helmSelector.Chart, helmSelector.Version, releaseName)
}

func (m *syncManager) LoadApplication(application *v1.AnyApplication) (types.GlobalApplication, error) {
	localApplication, err := m.loadLocalApplication(application)
	if err != nil {
		return nil, errors.Wrap(err, "Fail to create local application")
	}

	globalApplication := global.NewFromLocalApplication(localApplication, m.clock, application, m.config, m.log)
	return globalApplication, nil
}

func (m *syncManager) loadLocalApplication(application *v1.AnyApplication) (mo.Option[local.LocalApplication], error) {
	app, err := m.getOrRenderApp(application)
	if err != nil {
		return mo.None[local.LocalApplication](), err
	}
	expectedResources := app.resources
	availableResources := m.findAvailableApplicationResources(application)
	version := application.ResourceVersion
	localApplication, err := local.NewFromUnstructured(version, availableResources, expectedResources, m.config, m.clock, m.log)
	if err != nil {
		return mo.None[local.LocalApplication](), errors.Wrap(err, "Fail to create local application")
	}
	return localApplication, nil
}

func (m *syncManager) findAvailableApplicationResources(application *v1.AnyApplication) []*unstructured.Unstructured {
	instanceId := m.GetInstanceId(application)

	cachedResources := m.clusterCache.FindResources("", func(r *cache.Resource) bool {
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
