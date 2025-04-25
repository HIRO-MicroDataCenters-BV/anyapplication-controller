package job

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type LocalOperationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
	status        health.HealthStatusCode
	msg           string
}

func NewLocalOperationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *LocalOperationJob {
	return &LocalOperationJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		status:        health.HealthStatusProgressing,
	}
}

func (job *LocalOperationJob) Run(context AsyncJobContext) {
	// client := context.GetKubeClient()
	// ctx := context.GetGoContext()

}

func (job *LocalOperationJob) GetJobID() JobId {
	return JobId{}
}

func (job *LocalOperationJob) GetType() AsyncJobType {
	return AsyncJobTypeLocalOperation
}

func (job *LocalOperationJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.LocalConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}
