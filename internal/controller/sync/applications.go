package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	gitops_sync "github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cockroachdb/errors"
	mapset "github.com/deckarep/golang-set/v2"
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

type IsManagedResourceFunc func(*cache.Resource) bool

type applications struct {
	helmClient   helm.HelmClient
	charts       types.Charts
	kubeClient   client.Client
	clusterCache cache.ClusterCache
	appCache     sync.Map
	clock        clock.Clock
	config       *config.ApplicationRuntimeConfig
	gitOpsEngine engine.GitOpsEngine
	log          logr.Logger
}

func NewApplications(
	kubeClient client.Client,
	helmClient helm.HelmClient,
	charts types.Charts,
	clusterCache cache.ClusterCache,
	clock clock.Clock,
	config *config.ApplicationRuntimeConfig,
	gitOpsEngine engine.GitOpsEngine,
	logger logr.Logger,
) types.Applications {
	log := logger.WithName("SyncManager")

	return &applications{
		kubeClient:   kubeClient,
		charts:       charts,
		helmClient:   helmClient,
		clusterCache: clusterCache,
		appCache:     sync.Map{},
		clock:        clock,
		config:       config,
		gitOpsEngine: gitOpsEngine,
		log:          log,
	}
}

func (m *applications) GetTargetVersion(
	application *v1.AnyApplication,
) mo.Option[*types.SpecificVersion] {
	zone, found := application.Status.GetStatusFor(m.config.ZoneId)
	if !found {
		return mo.None[*types.SpecificVersion]()
	}
	version, err := types.NewSpecificVersion(zone.ChartVersion)
	if err != nil {
		m.log.Error(err, "Failed to parse chart version", "version", zone.ChartVersion)
		return mo.None[*types.SpecificVersion]()
	}
	return mo.Some(version)
}

func (m *applications) DetermineTargetVersion(
	application *v1.AnyApplication,
) (*types.SpecificVersion, error) {

	helmSource := application.Spec.Source.HelmSelector
	chartVersion, err := types.NewChartVersion(helmSource.Version)
	if err != nil {
		return nil, err
	}

	if chartKey, err := m.charts.AddAndGetLatest(helmSource.Chart, helmSource.Repository, chartVersion); err == nil {
		return &chartKey.Version, nil
	}
	return nil, err
}

func (m *applications) SyncVersion(
	ctx context.Context,
	application *v1.AnyApplication,
	version *types.SpecificVersion,
) (*types.SyncResult, error) {
	app, err := m.getOrRenderAppVersion(application, version)
	if err != nil {
		return types.NewSyncResult(), err
	}
	return m.sync(ctx, app)
}

func (m *applications) getOrRenderAppVersion(
	application *v1.AnyApplication,
	version *types.SpecificVersion,
) (*cachedApp, error) {
	appKey := m.getApplicationKey(application)
	chartKey := types.ChartKey{
		ChartId: types.NewChartId(application),
		Version: *version,
	}

	instances := m.getOrCreateInstances(appKey)
	uniqueConfiguration := m.buildInstanceKey(application, &chartKey)

	cachedApp, exists := instances.Get(&uniqueConfiguration)
	if !exists {
		newApp, err := m.render(application, &uniqueConfiguration)
		if err != nil {
			return nil, err
		}
		instances.Put(&uniqueConfiguration, newApp)
		cachedApp = newApp
	}
	return cachedApp, nil
}

func (m *applications) getOrCreateInstances(appKey string) *cachedInstances {
	instances, exists := m.appCache.Load(appKey)
	if !exists {
		instances = NewCachedInstances()
		m.appCache.Store(appKey, instances)
	}
	return instances.(*cachedInstances)
}

func (m *applications) buildInstanceKey(application *v1.AnyApplication, chartKey *types.ChartKey) instanceKey {
	return instanceKey{
		ChartKey: chartKey,
		Instance: &types.ApplicationInstance{
			InstanceId:  m.GetInstanceId(application),
			Name:        application.Name,
			Namespace:   application.Namespace,
			ReleaseName: application.Name,
			ValuesYaml:  application.Spec.Source.HelmSelector.Values,
		},
	}
}

func (m *applications) render(application *v1.AnyApplication, configuration *instanceKey) (*cachedApp, error) {

	renderedChart, err := m.charts.Render(configuration.ChartKey, configuration.Instance)
	if err != nil {
		return nil, errors.Wrap(err, "Fail to render chart")
	}

	revision, err := configuration.Revision()
	if err != nil {
		return nil, err
	}

	app := cachedApp{
		application:   application.DeepCopy(),
		chartKey:      configuration.ChartKey,
		instance:      configuration.Instance,
		revision:      revision,
		renderedChart: renderedChart,
	}
	return &app, nil
}

func (m *applications) sync(ctx context.Context, app *cachedApp) (*types.SyncResult, error) {

	syncResult := types.NewSyncResult()
	prune := true

	resourceSyncResults, err := m.gitOpsEngine.Sync(
		ctx,
		app.renderedChart.Resources,
		m.isManagedFunc(app.instance.InstanceId),
		app.revision,
		app.instance.Namespace,
		gitops_sync.WithPrune(prune),
		gitops_sync.WithLogr(m.log),
	)
	if err != nil {
		m.log.Error(err, "Failed to synchronize cluster state")
		return syncResult, errors.Wrap(err, "Failed to synchronize cluster state")
	}

	m.addAndLogResults(resourceSyncResults, syncResult)

	syncResult.AggregatedStatus = m.getAggregatedStatus(app)

	localApplications, err := m.loadLocalApplicationVersions(app.application)
	localApplication, exists := localApplications[app.chartKey.Version]
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

func (m *applications) addAndLogResults(resourceSyncResults []common.ResourceSyncResult, syncResult *types.SyncResult) {
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

func (m *applications) isManagedFunc(instanceId string) IsManagedResourceFunc {
	return func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		return r.Resource.GetLabels()[LABEL_INSTANCE_ID] == instanceId
	}
}

func (m *applications) GetAggregatedStatusVersion(
	application *v1.AnyApplication,
	version *types.SpecificVersion,
) *types.AggregatedStatus {
	app, err := m.getOrRenderAppVersion(application, version)
	if err != nil {
		m.log.Error(err, "Failed to get or render application")
	}
	return m.getAggregatedStatus(app)
}

func (m *applications) getAggregatedStatus(app *cachedApp) *types.AggregatedStatus {

	managedResources := m.findAvailableApplicationResources(app.application)
	managedResourcesByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for _, res := range managedResources {
		key := kube.GetResourceKey(res)
		managedResourcesByKey[key] = res
	}

	healthStatus := GetAggregatedStatus(app.renderedChart.Resources, managedResourcesByKey, m.log)
	return &types.AggregatedStatus{
		HealthStatus: healthStatus,
		ChartVersion: &app.chartKey.Version,
	}
}

func (m *applications) Cleanup(ctx context.Context, application *v1.AnyApplication) ([]*types.DeleteResult, error) {
	allDeployedVersions, err := m.GetAllPresentVersions(application)
	if err != nil {
		return nil, err
	}
	allVersions := allDeployedVersions.ToSlice()

	sort.Slice(allVersions, func(i, j int) bool {
		return !allVersions[i].IsNewerThan(allVersions[j])
	})

	deleteResults := make([]*types.DeleteResult, 0)
	for _, version := range allVersions {
		deleteResult, err := m.DeleteVersion(ctx, application, version)
		if err != nil {
			return nil, err
		}
		deleteResults = append(deleteResults, deleteResult)
	}
	return deleteResults, nil
}

func (m *applications) DeleteVersion(
	ctx context.Context,
	application *v1.AnyApplication,
	version *types.SpecificVersion,
) (*types.DeleteResult, error) {
	app, err := m.getOrRenderAppVersion(application, version)
	if err != nil {
		return nil, err
	}

	return m.deleteApp(ctx, app)
}

func (m *applications) deleteApp(ctx context.Context, app *cachedApp) (*types.DeleteResult, error) {
	deleteResult := &types.DeleteResult{}

	deleteResult.Total += len(app.renderedChart.Resources)

	managedResources := m.findAvailableApplicationResources(app.application)
	managedResourcesByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for _, res := range managedResources {
		key := kube.GetResourceKey(res)
		managedResourcesByKey[key] = res
	}
	// Deleting known managed resources
	for _, obj := range app.renderedChart.Resources {
		fullName := getFullName(obj)
		resourceKey := kube.GetResourceKey(obj)
		m.log.V(1).Info("Deleting resource", "Resource", fullName)
		live := managedResourcesByKey[resourceKey]

		if live == nil {
			deleteResult.Deleted += 1
		} else {
			err := m.kubeClient.Delete(ctx, live)
			if err != nil {
				deleteResult.DeleteFailed += 1
			} else {
				delete(managedResourcesByKey, resourceKey)
				deleteResult.Deleted += 1
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
			deleteResult.DeleteFailed += 1
		} else {
			delete(managedResourcesByKey, resourceKey)
			deleteResult.Deleted += 1
			m.log.V(1).Info("Deleted", "Resource", fullName)
		}
	}
	deleteResult.Version = &app.chartKey.Version
	deleteResult.ApplicationResourcesPresent = len(managedResourcesByKey) > 0
	return deleteResult, nil

}

func (m *applications) getApplicationKey(application *v1.AnyApplication) string {
	return fmt.Sprintf("%s-%s", application.Name, application.Namespace)
}

func (m *applications) GetInstanceId(application *v1.AnyApplication) string {
	appName := application.Name
	appNamespace := application.Namespace
	return fmt.Sprintf("%s-%s", appNamespace, appName)
}

func (m *applications) LoadApplication(application *v1.AnyApplication) (types.GlobalApplication, error) {
	localApplications, err := m.loadLocalApplicationVersions(application)
	if err != nil {
		return nil, errors.Wrap(err, "Fail to create local application")
	}

	newVersion := mo.None[*types.SpecificVersion]()

	activeVersionOpt := m.GetTargetVersion(application)
	targetVersion, err := m.DetermineTargetVersion(application)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to determine target version")
	}
	activeVersion, exists := activeVersionOpt.Get()
	if !exists || targetVersion.IsNewerThan(activeVersion) {
		newVersion = mo.Some(targetVersion)
	}

	globalApplication := global.NewFromLocalApplication(
		localApplications,
		activeVersionOpt,
		newVersion,
		m.clock,
		application,
		m.config,
		m.log,
	)
	return globalApplication, nil
}

func (m *applications) loadLocalApplicationVersions(
	application *v1.AnyApplication,
) (map[types.SpecificVersion]*local.LocalApplication, error) {

	availableResources := m.findAvailableApplicationResources(application)
	availableResourcesByVersion, err := splitResourcesByVersion(availableResources)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to split resources by version")
	}
	localApplications := make(map[types.SpecificVersion]*local.LocalApplication)

	for versionStr, resources := range availableResourcesByVersion {
		if len(resources) == 0 {
			continue
		}

		version, err := types.NewSpecificVersion(versionStr)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse version string %s", versionStr)
		}
		cachedApp, err := m.getOrRenderAppVersion(application, version)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get or render application for version %s", versionStr)
		}
		expectedResources := cachedApp.renderedChart.Resources

		localApplication, err := local.NewFromUnstructured(version, resources, expectedResources, m.config, m.clock, m.log)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create local application for version %s", version)
		}
		local, ok := localApplication.Get()
		if ok {
			localApplications[*version] = &local
		}

	}
	return localApplications, nil
}

func (m *applications) findAvailableApplicationResources(application *v1.AnyApplication) []*unstructured.Unstructured {
	instanceId := m.GetInstanceId(application)

	cachedResources := m.clusterCache.FindResources("", func(r *cache.Resource) bool {
		if r.Resource == nil {
			return false
		}
		labels := r.Resource.GetLabels()
		return labels != nil && labels[LABEL_INSTANCE_ID] == instanceId
	})

	resources := lo.Values(cachedResources)
	return lo.Map(resources, func(r *cache.Resource, index int) *unstructured.Unstructured { return r.Resource })
}

func (m *applications) GetAllPresentVersions(
	application *v1.AnyApplication,
) (mapset.Set[*types.SpecificVersion], error) {

	availableResources := m.findAvailableApplicationResources(application)
	availableResourcesByVersion, err := splitResourcesByVersion(availableResources)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to split resources by version")
	}

	deployedVersions := mapset.NewSet[*types.SpecificVersion]()
	for versionStr, resources := range availableResourcesByVersion {
		if len(resources) == 0 {
			continue
		}

		version, err := types.NewSpecificVersion(versionStr)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse version string %s", versionStr)
		}
		deployedVersions.Add(version)
	}

	return deployedVersions, nil
}

func getFullName(obj *unstructured.Unstructured) string {
	gvk := obj.GroupVersionKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()
	return fmt.Sprintf("%s/%s (%s)", namespace, name, gvk.Kind)
}

type cachedInstances struct {
	// map[uniqueConfiguration]*cachedApp
	configurations sync.Map
}

func NewCachedInstances() *cachedInstances {
	return &cachedInstances{
		configurations: sync.Map{},
	}
}

func (m *cachedInstances) Contains(config *instanceKey) bool {
	_, ok := m.configurations.Load(*config)
	return ok
}

func (c *cachedInstances) Put(
	config *instanceKey,
	app *cachedApp,
) {
	c.configurations.Store(*config, app)
}

func (c *cachedInstances) Get(
	config *instanceKey,
) (*cachedApp, bool) {
	if app, ok := c.configurations.Load(*config); ok {
		return app.(*cachedApp), true
	}
	return nil, false
}

func (c *cachedInstances) IsEmpty() bool {
	empty := true
	c.configurations.Range(func(_, _ any) bool {
		empty = false
		return false // stop iteration
	})
	return empty
}

type cachedApp struct {
	application   *v1.AnyApplication
	chartKey      *types.ChartKey
	instance      *types.ApplicationInstance
	renderedChart *types.RenderedChart
	revision      string
}

type instanceKey struct {
	ChartKey *types.ChartKey
	Instance *types.ApplicationInstance
}

func (c *instanceKey) Revision() (string, error) {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)

	return hex.EncodeToString(hash[:]), nil
}

func splitResourcesByVersion(resources []*unstructured.Unstructured) (map[string][]*unstructured.Unstructured, error) {
	resourcesByVersion := make(map[string][]*unstructured.Unstructured)

	for _, res := range resources {
		version, found, err := unstructured.NestedString(res.Object, "metadata", "labels", LABEL_CHART_VERSION)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get version for resource %s", res.GetName())
		}
		if !found {
			return nil, fmt.Errorf("resource %s does not have version label", res.GetName())
		}
		resourcesByVersion[version] = append(resourcesByVersion[version], res)
	}

	return resourcesByVersion, nil
}
