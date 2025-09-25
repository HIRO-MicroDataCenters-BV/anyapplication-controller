// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"hiro.io/anyapplication/internal/controller/fixture"
	"hiro.io/anyapplication/internal/httpapi/api"
)

func TestParseSpec_Nginx(t *testing.T) {

	resources := fixture.LoadYamlFixture("nginx.yaml")
	expected := fixture.LoadJSONFixture[api.ApplicationSpec]("nginx-spec.json")

	est := NewSpecParser("nginx", "default", resources)
	actual, err := est.Parse()
	assert.NoError(t, err)

	assert.Equal(t, &expected, actual)
}
