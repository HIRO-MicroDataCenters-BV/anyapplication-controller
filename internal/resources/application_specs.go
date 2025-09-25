// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/httpapi/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationSpecs struct {
	applications types.Applications
	kubeClient   client.Client
	log          logr.Logger
}

func NewApplicationSpecs(applications types.Applications, kubeClient client.Client, log logr.Logger) api.ApplicationSpecs {
	return &applicationSpecs{
		applications: applications,
		kubeClient:   kubeClient,
		log:          log,
	}
}

func (s *applicationSpecs) GetApplicationSpec(ctx context.Context, application *v1.AnyApplication) (*api.ApplicationSpec, error) {

	chart, err := s.applications.GetRenderedChart(application)
	if err != nil {
		return nil, err
	}

	specParser := NewSpecParser(application.Name, application.Namespace, chart.Resources)
	return specParser.Parse()
}
