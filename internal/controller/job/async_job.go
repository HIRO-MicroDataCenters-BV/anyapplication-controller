package job

import (
	"context"

	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/sync"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobType int

const (
	AsyncJobTypeUnknown AsyncJobType = iota
	AsyncJobTypeRelocate
	AsyncJobTypeOwnershipTransfer
	AsyncJobTypeLocalPlacement
	AsyncJobTypeLocalOperation
	AsyncJobTypeUndeploy
)

type JobId struct {
	JobType       AsyncJobType
	ApplicationId string
}

type AsyncJobContext interface {
	GetHelmClient() helm.HelmClient
	GetKubeClient() client.Client
	GetGoContext() context.Context
	GetSyncManager() sync.SyncManager
}

type AsyncJob interface {
	GetJobID() JobId
	GetType() AsyncJobType
	GetStatus() v1.ConditionStatus
	Run(context AsyncJobContext)
}

type AsyncJobFactory interface {
	CreateRelocationJob(application *v1.AnyApplication) AsyncJob
	CreateOnwershipTransferJob(application *v1.AnyApplication) AsyncJob
	CreateUndeployJob(application *v1.AnyApplication) AsyncJob
	CreateLocalPlacementJob(application *v1.AnyApplication) AsyncJob
	CreateOperationJob(application *v1.AnyApplication) AsyncJob
}

type AsyncJobs interface {
	Execute(job AsyncJob)
	GetCurrent(name types.NamespacedName) mo.Option[AsyncJob]
	Stop(job AsyncJob)
}
