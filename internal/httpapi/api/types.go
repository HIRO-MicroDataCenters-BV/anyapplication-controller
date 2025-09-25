// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
)

type ApplicationReports interface {
	Fetch(ctx context.Context, instanceId string, namespace string) (*ApplicationReport, error)
}

type ApplicationSpecs interface {
	GetApplicationSpec(ctx context.Context, namespace string, name string) (*ApplicationSpec, error)
}
