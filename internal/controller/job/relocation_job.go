package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type RelocationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
}

func NewRelocationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *RelocationJob {
	return &RelocationJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
	}
}

func (job *RelocationJob) Run(context AsyncJobContext) {

}

func (job *RelocationJob) GetJobID() JobId {
	return JobId{}
}

func (job *RelocationJob) GetType() AsyncJobType {
	return AsyncJobTypeRelocate
}

func (job *RelocationJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
	}
}
