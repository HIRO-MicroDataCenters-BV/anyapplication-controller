package types

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ChartKey struct {
	RepoUrl   string
	ChartName string
	Version   string
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
	AddChart(chartKey *ChartKey) error
}
