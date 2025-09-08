// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package job

import (
	"context"

	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobContextImpl struct {
	helmClient   helm.HelmClient
	kubeClient   client.Client
	ctx          context.Context
	applications types.Applications
}

func NewAsyncJobContext(helmClient helm.HelmClient, kubeClient client.Client, ctx context.Context, syncManager types.Applications) types.AsyncJobContext {
	return AsyncJobContextImpl{helmClient, kubeClient, ctx, syncManager}
}

func (a AsyncJobContextImpl) GetHelmClient() helm.HelmClient {
	return a.helmClient
}

func (a AsyncJobContextImpl) GetKubeClient() client.Client {
	return a.kubeClient
}

func (a AsyncJobContextImpl) GetGoContext() context.Context {
	return a.ctx
}

func (a AsyncJobContextImpl) WithCancel() (types.AsyncJobContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(a.ctx)
	return AsyncJobContextImpl{a.helmClient, a.kubeClient, ctx, a.applications}, cancel
}

func (a AsyncJobContextImpl) IsCancelled() bool {
	return types.IsCancelled(a.ctx)
}

func (a AsyncJobContextImpl) GetApplications() types.Applications {
	return a.applications
}
