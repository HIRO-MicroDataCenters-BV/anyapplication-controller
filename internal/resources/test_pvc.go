// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"
)

func TestParsePVC(t *testing.T) {
	// 	yaml := `
	// apiVersion: apps/v1
	// kind: Deployment
	// metadata:
	//   name: test
	// spec:
	//   replicas: 2
	//   template:
	//     spec:
	//       containers:
	//         - name: app
	//           resources:
	//             requests:
	//               cpu: "100m"
	//               memory: "128Mi"
	//             limits:
	//               cpu: "200m"
	//               memory: "256Mi"
	// `

	// est := NewPVCParser()
	// totals, err := est.Parse(yaml)
	// assert.NoError(t, err)

	// assert.Equal(t, "200m", totals.Requests.CPU.String())
	// assert.Equal(t, "256Mi", totals.Requests.Memory.String())
	// assert.Equal(t, "400m", totals.Limits.CPU.String())
	// assert.Equal(t, "512Mi", totals.Limits.Memory.String())
}
