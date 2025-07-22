package sync

import (
	"context"
	"sync"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cockroachdb/errors"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
)

type ChartsOptions struct {
	SyncPeriod time.Duration
}

type charts struct {
	ctx        context.Context
	charts     sync.Map
	helmClient helm.HelmClient
	options    *ChartsOptions
}

func NewCharts(
	ctx context.Context,
	helmClient helm.HelmClient,
	options *ChartsOptions,
) types.Charts {
	return &charts{
		ctx:        ctx,
		charts:     sync.Map{},
		helmClient: helmClient,
		options:    options,
	}
}

func (c *charts) AddChart(chartName string, repoUrl string, version types.ChartVersion) (*types.ChartKey, error) {
	chartId := types.ChartId{RepoUrl: repoUrl, ChartName: chartName}
	chartKey := &types.ChartKey{ChartId: chartId, Version: version}
	versions, err := c.GetOrCreateVersions(&chartKey.ChartId)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get or create chart versions")
	}

	if !versions.Exists(chartKey.Version) {

	}
	return nil, nil
}

func (c *charts) GetOrCreateVersions(chartId *types.ChartId) (*ChartVersions, error) {
	versions, exists := c.charts.Load(chartId)
	if !exists {
		repoName, err := c.helmClient.AddOrUpdateChartRepo(chartId.RepoUrl)
		if err != nil {
			return nil, err
		}
		versions = &ChartVersions{repoName: repoName, charts: sync.Map{}}
		c.charts.Store(chartId, versions)
	}

	return versions.(*ChartVersions), nil
}

func (c *charts) RunSynchronization() {
	ticker := time.NewTicker(c.options.SyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			isCompleted := c.runSyncCycle()
			if isCompleted {
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *charts) runSyncCycle() bool {
	c.charts.Range(func(key, value any) bool {
		chartId := key.(*types.ChartId)
		versions := value.(*ChartVersions)
		c.probeNewVersion(chartId, versions)
		return true
	})
	return false
}

func (c *charts) probeNewVersion(chartId *types.ChartId, versions *ChartVersions) {
	// TODO
}

func (c *charts) Render(chartKey *types.ChartKey, instance *types.ApplicationInstance) (*types.RenderedChart, error) {

	labels := map[string]string{
		"dcp.hiro.io/managed-by":  "dcp",
		"dcp.hiro.io/instance-id": instance.InstanceId,
	}

	template, err := c.helmClient.Template(&helm.TemplateArgs{
		ReleaseName: instance.ReleaseName,
		RepoUrl:     chartKey.ChartId.RepoUrl,
		ChartName:   chartKey.ChartId.ChartName,
		Namespace:   instance.Namespace,
		Version:     chartKey.Version.ToString(),
		ValuesYaml:  instance.ValuesYaml,
		Labels:      labels,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Helm template failure")
	}
	resources, err := kube.SplitYAML([]byte(template))
	if err != nil {
		return nil, errors.Wrap(err, "Fail to split YAML")
	}
	return &types.RenderedChart{
		Key:       *chartKey,
		Instance:  *instance,
		Resources: resources,
	}, nil
}

type ChartVersions struct {
	charts   sync.Map
	repoName string
}

func (cv *ChartVersions) AddVersion(version types.ChartVersion) {
	cv.charts.Store(version.ToString(), version)
}

func (cv *ChartVersions) HasNewerVersion(version types.ChartVersion) (*types.ChartVersion, bool) {
	var hasNewerVersion = false
	var newerChartVersion types.ChartVersion
	cv.charts.Range(func(key, value any) bool {
		hasNewerVersion = true
		newerChartVersion = value.(types.ChartVersion)
		return true
	})
	if hasNewerVersion {
		return &newerChartVersion, true
	}
	return nil, false
}

func (cv *ChartVersions) Exists(version types.ChartVersion) bool {
	_, found := cv.charts.Load(version.ToString())
	return found
}
