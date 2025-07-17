package types

import (
	"context"

	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobType string

const (
	AsyncJobTypeUnknown           AsyncJobType = "Unknown"
	AsyncJobTypeDeploy            AsyncJobType = "Deployment"
	AsyncJobTypeOwnershipTransfer AsyncJobType = "OwnershipTransfer"
	AsyncJobTypeLocalPlacement    AsyncJobType = "Placement"
	AsyncJobTypeLocalOperation    AsyncJobType = "Local"
	AsyncJobTypeUndeploy          AsyncJobType = "Undeployment"
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
	WithCancel() (AsyncJobContext, context.CancelFunc)
}

type AsyncJob interface {
	GetJobID() JobId
	GetType() AsyncJobType
	GetStatus() v1.ConditionStatus
	Run(context AsyncJobContext)
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

func IsCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		// Context is canceled
		return true
	default:
		// Context is still active
		return false
	}
}
