// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/httpapi/api"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (s *applicationSpecs) GetApplicationSpec(ctx context.Context, namespace string, name string) (*api.ApplicationSpec, error) {
	application, err := s.fetchApplication(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	chart, err := s.applications.GetRenderedChart(application)
	if err != nil {
		return nil, err
	}

	specParser := NewSpecParser(name, namespace, chart.Resources)
	return specParser.Parse()
}

func (s *applicationSpecs) fetchApplication(ctx context.Context, name string, namespace string) (*v1.AnyApplication, error) {
	resource := &v1.AnyApplication{}
	namespacedName := client.ObjectKey{Name: name, Namespace: namespace}
	if err := s.kubeClient.Get(ctx, namespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			s.log.Info("AnyApplication resource not found. Ignoring since object must be deleted", "name", name, "namespace", namespace)
			return nil, err
		}
		s.log.Error(err, "Unable to get AnyApplication ", "name", name, "namespace", namespace)
		return nil, err
	}
	return resource, nil
}
