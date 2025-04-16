package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AsyncJobType int

const (
	AsyncJobTypeUnknown AsyncJobType = iota
	AsyncJobTypeRelocate
	AsyncJobTypeOwnershipTransfer
	AsyncJobTypeUndeploy
)

type JobId struct {
	JobType       AsyncJobType
	ApplicationId string
}

type AsyncJobContext interface {
	GetHelmClient() helm.HelmClient
	GetKubeClient() client.Client
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
}
