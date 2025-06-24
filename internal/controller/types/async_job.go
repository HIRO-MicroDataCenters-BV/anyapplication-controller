package types

import (
	"context"

	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobType int

const (
	AsyncJobTypeUnknown AsyncJobType = iota
	AsyncJobTypeDeploy
	AsyncJobTypeOwnershipTransfer
	AsyncJobTypeLocalPlacement
	AsyncJobTypeLocalOperation
	AsyncJobTypeUndeploy
)

type JobId struct {
	JobType       AsyncJobType
	ApplicationId ApplicationId
}

type ApplicationId struct {
	Name      string
	Namespace string
}

type AsyncJobContext interface {
	GetHelmClient() helm.HelmClient
	GetKubeClient() client.Client
	GetGoContext() context.Context
	GetSyncManager() SyncManager
}

type AsyncJob interface {
	GetJobID() JobId
	GetType() AsyncJobType
	GetStatus() v1.ConditionStatus
	Run(context AsyncJobContext)
	Stop()
}

type AsyncJobFactory interface {
	CreateLocalPlacementJob(application *v1.AnyApplication) AsyncJob
	CreateDeployJob(application *v1.AnyApplication) AsyncJob
	CreateUndeployJob(application *v1.AnyApplication) AsyncJob
	CreateOperationJob(application *v1.AnyApplication) AsyncJob
	CreateOnwershipTransferJob(application *v1.AnyApplication) AsyncJob
}

type AsyncJobs interface {
	Execute(job AsyncJob)
	GetCurrent(id ApplicationId) mo.Option[AsyncJob]
	Stop(id ApplicationId)
}
