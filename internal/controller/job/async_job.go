package job

import (
	v1 "hiro.io/anyapplication/api/v1"
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

type AsyncJob interface {
	GetJobID() JobId
	GetType() AsyncJobType
	GetStatus() v1.ConditionStatus
	Run()
}

type AsyncJobFactory interface {
	CreateRelocationJob(application *v1.AnyApplication) AsyncJob
	CreateOnwershipTransferJob(application *v1.AnyApplication) AsyncJob
	CreateUndeployJob(application *v1.AnyApplication) AsyncJob
}
