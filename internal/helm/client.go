package helm

import (
	"math/rand"

	semver "github.com/Masterminds/semver/v3"
)

type HelmClient interface {
	AddOrUpdateChartRepo(repoURL string) (string, error)
	SyncRepositories() error
	FetchVersions(repoURL string, chartName string) ([]*semver.Version, error)
	Template(args *TemplateArgs) (string, error)
}

const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandClient() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
