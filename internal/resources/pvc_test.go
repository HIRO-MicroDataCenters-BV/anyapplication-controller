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

func TestParsePVC(t *testing.T) {
	yamlString := `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
  - ReadWriteOnce
  storageClassName: ssd
  resources:
    requests:
      storage: 5Gi
    limits:
      storage: 10Gi`
	u := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlString), u)
	assert.NoError(t, err)

	est := NewPVCParser()
	pvcResources, err := est.Parse(u)
	assert.NoError(t, err)

	assert.Equal(t, &api.PVCResources{
		Id:           api.ResourceId{Name: "my-pvc", Namespace: ""},
		Limits:       map[string]string{"storage": "10Gi"},
		Replica:      1,
		Requests:     map[string]string{"storage": "5Gi"},
		StorageClass: "ssd",
	}, pvcResources)
}
