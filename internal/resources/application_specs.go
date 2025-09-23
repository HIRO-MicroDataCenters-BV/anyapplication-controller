// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"hiro.io/anyapplication/internal"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/httpapi/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationSpecs struct {
	charts     types.Charts
	kubeClient client.Client
}

func NewApplicationSpecs(charts types.Charts, kubeClient client.Client) internal.ApplicationSpecs {
	return &applicationSpecs{
		charts:     charts,
		kubeClient: kubeClient,
	}
}

func (s *applicationSpecs) GetApplicationSpec(ctx context.Context, namespace string, name string) (*api.ApplicationSpec, error) {

	return nil, nil
}
