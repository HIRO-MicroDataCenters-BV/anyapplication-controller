package types

import (
	semver "github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ChartId struct {
	RepoUrl   string
	ChartName string
}

type ChartKey struct {
	ChartId ChartId
	Version ChartVersion
}

type ApplicationInstance struct {
	InstanceId  string
	Name        string
	Namespace   string
	ReleaseName string
	ValuesYaml  string
}

type RenderedChart struct {
	Key       ChartKey
	Instance  ApplicationInstance
	Resources []*unstructured.Unstructured
}

type Chart struct {
}

type Charts interface {
	RunSynchronization()
	Render(chartKey *ChartKey, instance *ApplicationInstance) (*RenderedChart, error)
	AddChart(chartName string, repoUrl string, version ChartVersion) (*ChartKey, error)
}

type ChartVersion interface {
	ToString() string
}

type specificVersion struct {
	version semver.Version
}

func NewChartVersion(version string) (ChartVersion, error) {
	v, err := semver.NewVersion(version)
	if err == nil {
		return &specificVersion{version: *v}, nil
	}
	c, err := semver.NewConstraint(version)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid chart version: %s", version)
	}
	return &versionRange{constraint: *c}, nil
}

func (v *specificVersion) ToString() string {
	return v.version.String()
}

type versionRange struct {
	constraint semver.Constraints
}

func (v *versionRange) ToString() string {
	return v.constraint.String()
}
