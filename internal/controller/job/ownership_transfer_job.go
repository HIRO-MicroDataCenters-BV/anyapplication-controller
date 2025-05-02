package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type OwnershipTransferJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.OwnershipTransferStatus
	clock         clock.Clock
	jobId         JobId
}

func NewOwnershipTransferJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *OwnershipTransferJob {
	jobId := JobId{
		JobType: AsyncJobTypeLocalOperation,
		ApplicationId: ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &OwnershipTransferJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		status:        v1.OwnershipTransferPulling,
		clock:         clock,
		jobId:         jobId,
	}
}

func (job *OwnershipTransferJob) Run(context AsyncJobContext) {

}

func (job *OwnershipTransferJob) GetJobID() JobId {
	return job.jobId
}

func (job *OwnershipTransferJob) GetType() AsyncJobType {
	return AsyncJobTypeOwnershipTransfer
}

func (job *OwnershipTransferJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.OwnershipTransferConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
	}
}

func (job *OwnershipTransferJob) Stop() {}
