package job

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
)

type AsyncJobType int

const (
	AsyncJobTypeUnknown AsyncJobType = iota
	AsyncJobTypeFetch
)

type AsyncJobState int

const (
	AsyncJobStateUnknown AsyncJobState = iota
	AsyncJobStateNew
	AsyncJobStateExecuting
	AsyncJobStateCompleted
)

type AsyncJobStatus int

const (
	AsyncJobStatusUnknown AsyncJobStatus = iota
	AsyncJobStatusSuccess
	AsyncJobStatusFailure
	AsyncJobStatusCancelled
)

type AsyncJob interface {
	GetJobID() int
	GetType() AsyncJobType
	GetState() AsyncJobState
	GetStatus() v1.ConditionStatus
	GetCompletionStatus() mo.Option[AsyncJobStatus]
	GetCompletionComment() mo.Option[string]
	Run()
}
