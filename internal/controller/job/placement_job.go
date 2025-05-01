package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/types"
)

type LocalPlacementJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
	status        v1.PlacementStatus
	msg           string
	jobId         types.JobId
}

func NewLocalPlacementJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *LocalPlacementJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &LocalPlacementJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		status:        v1.PlacementStatusInProgress,
		jobId:         jobId,
	}
}

func (job *LocalPlacementJob) Run(context types.AsyncJobContext) {
	client := context.GetKubeClient()
	ctx := context.GetGoContext()

	job.status = v1.PlacementStatusDone

	err := AddOrUpdateStatusCondition(ctx, client, job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.PlacementStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalPlacementJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *LocalPlacementJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeLocalPlacement
}

func (job *LocalPlacementJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.PlacementConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *LocalPlacementJob) Stop() {}
