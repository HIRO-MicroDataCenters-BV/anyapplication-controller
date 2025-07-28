package types

import (
	"encoding/json"

	semver "github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ChartId struct {
	RepoUrl   string
	ChartName string
}

func NewChartId(application *v1.AnyApplication) ChartId {
	return ChartId{
		RepoUrl:   application.Spec.Source.HelmSelector.Repository,
		ChartName: application.Spec.Source.HelmSelector.Chart,
	}
}

type ChartKey struct {
	ChartId ChartId
	Version SpecificVersion
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

type Chart struct{}

type Charts interface {
	RunSynchronization()
	RunSyncCycle()
	Render(chartKey *ChartKey, instance *ApplicationInstance) (*RenderedChart, error)
	AddAndGetLatest(chartName string, repoUrl string, version ChartVersion) (*ChartKey, error)
	RegisterChart(chartName string, repoUrl string) error
}

type ChartVersion interface {
	ToString() string
}

type SpecificVersion struct {
	version semver.Version
}

func NewFromSemantic(version *semver.Version) (*SpecificVersion, error) {
	return &SpecificVersion{version: *version}, nil
}

func NewSpecificVersion(version string) (*SpecificVersion, error) {
	v, err := semver.NewVersion(version)
	if err == nil {
		return &SpecificVersion{version: *v}, nil
	}
	return nil, errors.Wrapf(err, "invalid specific version: %s", version)
}

func (v *SpecificVersion) ToString() string {
	return v.version.String()
}

func (v *SpecificVersion) IsNewerThan(other *SpecificVersion) bool {
	return v.version.GreaterThan(&other.version)
}

func (v *SpecificVersion) UnmarshalJSON(data []byte) error {
	var versionString string
	if err := json.Unmarshal(data, &versionString); err != nil {
		return errors.Wrap(err, "failed to unmarshal SpecificVersion")
	}
	version, err := semver.NewVersion(versionString)
	if err != nil {
		return errors.Wrap(err, "failed to parse SpecificVersion")
	}
	v.version = *version
	return nil
}

func (v *SpecificVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.ToString())
}

type VersionRange struct {
	constraint semver.Constraints
}

func (v *VersionRange) Contains(version *SpecificVersion) bool {
	return v.constraint.Check(&version.version)
}

func (v *VersionRange) ToString() string {
	return v.constraint.String()
}

func NewChartVersion(version string) (ChartVersion, error) {
	v, err := semver.NewVersion(version)
	if err == nil {
		return &SpecificVersion{version: *v}, nil
	}
	c, err := semver.NewConstraint(version)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid chart version: %s", version)
	}
	return &VersionRange{constraint: *c}, nil
}
