// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cockroachdb/errors"
	"github.com/go-logr/logr"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
)

const (
	LABEL_MANAGED_BY           = "dcp.hiro.io/managed-by"
	LABEL_CHART_VERSION        = "dcp.hiro.io/chart-version"
	LABEL_INSTANCE_ID          = "dcp.hiro.io/instance-id"
	LABEL_VALUE_MANAGED_BY_DCP = "dcp"
)

type ChartsOptions struct {
	SyncPeriod time.Duration
}

type charts struct {
	ctx        context.Context
	charts     sync.Map
	helmClient helm.HelmClient
	options    *ChartsOptions
	logger     logr.Logger
}

func NewCharts(
	ctx context.Context,
	helmClient helm.HelmClient,
	options *ChartsOptions,
	logger logr.Logger,
) types.Charts {
	return &charts{
		ctx:        ctx,
		charts:     sync.Map{},
		helmClient: helmClient,
		logger:     logger,
		options:    options,
	}
}

func (c *charts) AddAndGetLatest(chartName string, repoUrl string, chartVersion types.ChartVersion) (*types.ChartKey, error) {

	if err := c.RegisterChart(chartName, repoUrl); err != nil {
		return nil, err
	}

	return c.pullVersion(chartName, repoUrl, chartVersion)
}

func (c *charts) pullVersion(chartName string, repoUrl string, chartVersion types.ChartVersion) (*types.ChartKey, error) {
	specificVersion, isSpecificVersion := chartVersion.(*types.SpecificVersion)
	versionRange, _ := chartVersion.(*types.VersionRange)
	chartId := types.ChartId{RepoUrl: repoUrl, ChartName: chartName}

	chartVersions, err := c.getOrCreateVersions(&chartId)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get or create chart versions")
	}
	if chartVersions.isEmpty() {
		c.updateAvailableVersions(&chartId, chartVersions)
	}
	if isSpecificVersion {
		if chartVersions.Exists(specificVersion) {
			return &types.ChartKey{
				ChartId: chartId,
				Version: *specificVersion,
			}, nil
		}
		return nil, errors.Errorf("Specific version %s not found for chart %s", specificVersion.ToString(), chartName)
	} else {
		if specificVersion, found := chartVersions.getLatestFor(versionRange); found {
			return &types.ChartKey{
				ChartId: chartId,
				Version: *specificVersion,
			}, nil
		}
		return nil, errors.Errorf("Latest version not found for chart %s", chartName)
	}
}

func (c *charts) RegisterChart(chartName string, repoUrl string) error {
	chartId := types.ChartId{RepoUrl: repoUrl, ChartName: chartName}

	_, err := c.getOrCreateVersions(&chartId)
	if err != nil {
		return errors.Wrap(err, "Failed to get or create chart versions")
	}
	return nil
}

func (c *charts) getOrCreateVersions(chartId *types.ChartId) (*ChartVersions, error) {
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
			c.RunSyncCycle()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *charts) RunSyncCycle() {
	if err := c.helmClient.SyncRepositories(); err != nil {
		c.logger.Error(err, "Failed to sync Helm repositories")

	}
	c.charts.Range(func(key, value any) bool {
		chartId := key.(*types.ChartId)
		versions := value.(*ChartVersions)
		c.updateAvailableVersions(chartId, versions)
		return true
	})
}

func (c *charts) updateAvailableVersions(chartId *types.ChartId, versions *ChartVersions) {
	semanticVersions, err := c.helmClient.FetchVersions(chartId.RepoUrl, chartId.ChartName)
	if err != nil {
		c.logger.Error(err, "Failed to fetch versions for chart", "chartId", chartId)
	}
	versions.UpdateVersions(semanticVersions)
}

func (c *charts) Render(chartKey *types.ChartKey, instance *types.ApplicationInstance) (*types.RenderedChart, error) {

	labels := map[string]string{
		LABEL_MANAGED_BY:    LABEL_VALUE_MANAGED_BY_DCP,
		LABEL_CHART_VERSION: chartKey.Version.ToString(),
		LABEL_INSTANCE_ID:   instance.InstanceId,
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

func (cv *ChartVersions) AddVersion(version *types.SpecificVersion) {
	cv.charts.Store(version.ToString(), version)
}

func (cv *ChartVersions) GetNewerVersion(version *types.SpecificVersion) (*types.SpecificVersion, bool) {
	var hasNewerVersion = false
	var newerChartVersion *types.SpecificVersion

	cv.charts.Range(func(key, value any) bool {
		hasNewerVersion = true
		chartVersionIter := value.(*types.SpecificVersion)
		if chartVersionIter.IsNewerThan(version) {
			hasNewerVersion = true
			newerChartVersion = chartVersionIter
		}
		return true
	})
	if hasNewerVersion {
		return newerChartVersion, true
	}
	return nil, false
}

func (cv *ChartVersions) UpdateVersions(semanticVersions []*semver.Version) {
	for _, version := range semanticVersions {
		specificVersion, err := types.NewFromSemantic(version)
		if err != nil {
			fmt.Printf("Failed to parse semantic version %s, %s", version.String(), err.Error())
			continue
		}
		cv.AddVersion(specificVersion)
	}
}

func (cv *ChartVersions) Exists(version *types.SpecificVersion) bool {
	_, found := cv.charts.Load(version.ToString())
	return found
}

func (cv *ChartVersions) isEmpty() bool {
	isEmpty := true
	cv.charts.Range(func(key, value any) bool {
		isEmpty = false
		return false // stop at first element
	})
	return isEmpty
}

func (cv *ChartVersions) getLatestFor(versionRange *types.VersionRange) (*types.SpecificVersion, bool) {
	if versionRange == nil {
		return nil, false
	}

	var latestChartVersion *types.SpecificVersion

	cv.charts.Range(func(key, value any) bool {
		chartVersionIter := value.(*types.SpecificVersion)
		if !versionRange.Contains(chartVersionIter) {
			return true
		}
		if latestChartVersion == nil {
			latestChartVersion = chartVersionIter
		} else if latestChartVersion != nil && chartVersionIter.IsNewerThan(latestChartVersion) {
			latestChartVersion = chartVersionIter
		}
		return true
	})
	if latestChartVersion != nil {
		return latestChartVersion, true
	}
	return nil, false

}
