package job

import (
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobContextImpl struct {
}

func NewAsyncJobContext() AsyncJobContext {
	return AsyncJobContextImpl{}
}

// GetHelmClient implements AsyncJobContext.
func (a AsyncJobContextImpl) GetHelmClient() helm.HelmClient {
	panic("unimplemented")
}

// GetKubeClient implements AsyncJobContext.
func (a AsyncJobContextImpl) GetKubeClient() client.Client {
	panic("unimplemented")
}
