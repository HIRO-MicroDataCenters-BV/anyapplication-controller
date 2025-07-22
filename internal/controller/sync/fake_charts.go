package sync

import "hiro.io/anyapplication/internal/controller/types"

type FakeCharts struct {
}

func NewFakeCharts() *FakeCharts {
	return &FakeCharts{}
}

func (f *FakeCharts) RunSynchronization() {
	// No-op for fake implementation
}
func (f *FakeCharts) Render(chartKey *types.ChartKey, instance *types.ApplicationInstance) (*types.RenderedChart, error) {
	// Return a dummy rendered chart for testing purposes
	return &types.RenderedChart{
		Key:      *chartKey,
		Instance: *instance,
	}, nil
}
func (f *FakeCharts) AddChart(chartKey *types.ChartKey) error {
	// Simulate adding a chart by returning nil
	return nil
}
