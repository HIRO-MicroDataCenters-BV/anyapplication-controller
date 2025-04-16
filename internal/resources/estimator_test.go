package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateFromYAML_BasicDeployment(t *testing.T) {
	yaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
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

	est := NewResourceEstimator()
	totals, err := est.EstimateFromYAML(yaml)
	assert.NoError(t, err)

	assert.Equal(t, "200m", totals.Requests.CPU.String())
	assert.Equal(t, "256Mi", totals.Requests.Memory.String())
	assert.Equal(t, "400m", totals.Limits.CPU.String())
	assert.Equal(t, "512Mi", totals.Limits.Memory.String())
}

func TestEstimateFromYAML_WithInitContainer(t *testing.T) {
	yaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
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

	est := NewResourceEstimator()
	totals, err := est.EstimateFromYAML(yaml)
	assert.NoError(t, err)

	assert.Equal(t, "200m", totals.Requests.CPU.String()) // 150 + 50
	assert.Equal(t, "320Mi", totals.Requests.Memory.String())
}

func TestEstimateFromYAML_InvalidYAML(t *testing.T) {
	yaml := `this is not valid yaml`

	est := NewResourceEstimator()
	_, err := est.EstimateFromYAML(yaml)
	assert.Error(t, err)
}
