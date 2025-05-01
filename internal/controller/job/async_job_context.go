package job

import (
	"context"

	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobContextImpl struct {
	helmClient  helm.HelmClient
	kubeClient  client.Client
	ctx         context.Context
	syncManager types.SyncManager
}

func NewAsyncJobContext(helmClient helm.HelmClient, kubeClient client.Client, ctx context.Context, syncManager types.SyncManager) types.AsyncJobContext {
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

func (a AsyncJobContextImpl) GetSyncManager() types.SyncManager {
	return a.syncManager
}
