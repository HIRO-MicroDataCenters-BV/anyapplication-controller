// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestEstimateFromYAML_BasicDeployment(t *testing.T) {
	yamlString := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: ns
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: app
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "200m"
              memory: "256Mi"
`
	u := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlString), u)
	assert.NoError(t, err)

	est := NewWorkloadParser()
	totals, pvc, err := est.Parse(u)
	assert.NoError(t, err)

	assert.Equal(t, &api.PodResources{
		Id:       api.ResourceId{Name: "test", Namespace: "ns"},
		Limits:   map[string]string{"cpu": "200m", "memory": "256Mi"},
		Replica:  2,
		Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
	}, totals)

	assert.Equal(t, []api.PVCResources(nil), pvc)

}

func TestEstimateFromYAML_WithInitContainer(t *testing.T) {
	yamlString := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: ns
spec:
  replicas: 1
  template:
    spec:
      initContainers:
        - name: init
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
      containers:
        - name: app
          resources:
            requests:
              cpu: "150m"
              memory: "256Mi"
`
	u := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlString), u)
	assert.NoError(t, err)

	est := NewWorkloadParser()
	totals, pvc, err := est.Parse(u)
	assert.NoError(t, err)

	assert.Equal(t, &api.PodResources{
		Id:       api.ResourceId{Name: "test", Namespace: "ns"},
		Limits:   map[string]string{},
		Replica:  1,
		Requests: map[string]string{"cpu": "200m", "memory": "320Mi"},
	}, totals)

	assert.Equal(t, []api.PVCResources(nil), pvc)
}

func TestParseYAML_StatefullSetWithPVC(t *testing.T) {
	yamlString := `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
spec:
  selector:
    matchLabels:
      app: nginx # has to match .spec.template.metadata.labels
  serviceName: "nginx"
  replicas: 3 # by default is 1
  minReadySeconds: 10 # by default is 0
  template:
    metadata:
      labels:
        app: nginx # has to match .spec.selector.matchLabels
    spec:
      terminationGracePeriodSeconds: 10
      initContainers:
        - name: init
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
      containers:
        - name: app
          resources:
            requests:
              cpu: "150m"
              memory: "256Mi"
  volumeClaimTemplates:
  - metadata:
      name: www
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: "my-storage-class"
      resources:
        requests:
          storage: 1Gi
`
	u := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlString), u)
	assert.NoError(t, err)

	est := NewWorkloadParser()
	totals, pvc, err := est.Parse(u)
	assert.NoError(t, err)

	assert.Equal(t, &api.PodResources{
		Id:       api.ResourceId{Name: "web", Namespace: ""},
		Limits:   map[string]string{},
		Replica:  3,
		Requests: map[string]string{"cpu": "200m", "memory": "320Mi"},
	}, totals)

	assert.Equal(t, []api.PVCResources{
		{
			Id: api.ResourceId{
				Name:      "www",
				Namespace: "",
			},
			Limits:  map[string]string{},
			Replica: 3,
			Requests: map[string]string{
				"storage": "1Gi",
			},
			StorageClass: "my-storage-class",
		},
	}, pvc)

}
