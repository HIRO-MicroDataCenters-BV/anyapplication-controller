package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type LocalPlacementJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
}

func NewLocalPlacementJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) LocalPlacementJob {
	return LocalPlacementJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
	}
}

func (job LocalPlacementJob) Run(context AsyncJobContext) {

}

func (job LocalPlacementJob) GetJobID() JobId {
	return JobId{}
}

func (job LocalPlacementJob) GetType() AsyncJobType {
	return AsyncJobTypeLocalPlacement
}

func (job LocalPlacementJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.PlacementConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(v1.PlacementStatusInProgress),
		LastTransitionTime: job.clock.NowTime(),
	}
}
