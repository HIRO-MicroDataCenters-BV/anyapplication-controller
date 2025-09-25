// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"

	v1 "hiro.io/anyapplication/api/v1"
)

type ApplicationReports interface {
	Fetch(ctx context.Context, instanceId string, namespace string) (*ApplicationReport, error)
}

type ApplicationSpecs interface {
	GetApplicationSpec(ctx context.Context, application *v1.AnyApplication) (*ApplicationSpec, error)
}
