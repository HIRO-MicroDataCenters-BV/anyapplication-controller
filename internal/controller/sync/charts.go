package sync

import (
	"context"
	"time"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
)

type ChartsOptions struct {
	syncPeriod time.Duration
}

type charts struct {
	ctx        context.Context
	helmClient helm.HelmClient
	options    *ChartsOptions
}

func (c *charts) NewCharts(
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

func (c *charts) AddChart(chart *any) error {
	// Logic to add a chart
	return nil
}

func (c *charts) GetChart(application *v1.AnyApplication) (*types.Chart, error) {
	// Logic to get a chart
	return nil, nil
}

func (c *charts) RunSynchronization() {
	ticker := time.NewTicker(c.options.syncPeriod)
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
