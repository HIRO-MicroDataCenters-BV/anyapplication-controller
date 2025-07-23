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
func (f *FakeCharts) AddAndGetLatest(chartName string, repoUrl string, chartVersion types.ChartVersion) (*types.ChartKey, error) {

	version, err := types.NewSpecificVersion(chartVersion.ToString())
	if err != nil {
		return nil, err
	}
	return &types.ChartKey{
		ChartId: types.ChartId{
			RepoUrl:   repoUrl,
			ChartName: chartName,
		},
		Version: *version,
	}, nil
}
