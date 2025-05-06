package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

type UndeployJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
	jobId         types.JobId
}

func NewUndeployJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *UndeployJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &UndeployJob{
		status:        v1.RelocationStatusUndeploy,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
	}
}

func (job *UndeployJob) Run(context types.AsyncJobContext) {
	ctx := context.GetGoContext()

	syncManager := context.GetSyncManager()
	_, err := syncManager.Delete(ctx, job.application)

	if err != nil {
		job.Fail(context, err.Error())
		return
	} else {
		job.Success(context)
	}
}

func (job *UndeployJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.RelocationStatusFailure
	err := status.AddOrUpdateCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *UndeployJob) Success(context types.AsyncJobContext) {
	job.status = v1.RelocationStatusDone
	err := status.AddOrUpdateCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *UndeployJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *UndeployJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeUndeploy
}

func (job *UndeployJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *UndeployJob) Stop() {}
