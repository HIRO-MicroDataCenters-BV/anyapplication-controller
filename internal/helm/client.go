package helm

import (
	semver "github.com/Masterminds/semver/v3"
)

type HelmClient interface {
	AddOrUpdateChartRepo(repoURL string) (string, error)
	SyncRepositories()
	FetchVersions(repoURL string, chartName string) ([]*semver.Version, error)
	Template(args *TemplateArgs) (string, error)
}
