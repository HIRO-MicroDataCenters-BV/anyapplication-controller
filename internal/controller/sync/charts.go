package sync

import (
	"context"
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
		helmClient: helmClient,
		options:    options,
	}
}

func (c *charts) AddChart(chartKey *types.ChartKey) error {
	// Logic to add a chart
	return nil
}

func (c *charts) GetChart(chartKey *types.ChartKey) (*types.Chart, error) {
	// Logic to get a chart
	return nil, nil
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

	return false
}

func (c *charts) Render(chartKey *types.ChartKey, instance *types.ApplicationInstance) (*types.RenderedChart, error) {

	labels := map[string]string{
		"dcp.hiro.io/managed-by":  "dcp",
		"dcp.hiro.io/instance-id": instance.InstanceId,
	}

	template, err := c.helmClient.Template(&helm.TemplateArgs{
		ReleaseName: instance.ReleaseName,
		RepoUrl:     chartKey.RepoUrl,
		ChartName:   chartKey.ChartName,
		Namespace:   instance.Namespace,
		Version:     chartKey.Version,
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
