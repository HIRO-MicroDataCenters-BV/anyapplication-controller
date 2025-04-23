package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type UndeployJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
}

func NewUndeployJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *UndeployJob {
	return &UndeployJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
	}
}

func (job *UndeployJob) Run(context AsyncJobContext) {

}

func (job *UndeployJob) GetJobID() JobId {
	return JobId{}
}

func (job *UndeployJob) GetType() AsyncJobType {
	return AsyncJobTypeUndeploy
}

func (job *UndeployJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
	}
}
