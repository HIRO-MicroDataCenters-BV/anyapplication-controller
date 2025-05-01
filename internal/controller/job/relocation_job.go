package job

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/types"
)

type RelocationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
	jobId         types.JobId
}

func NewRelocationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *RelocationJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &RelocationJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
	}
}

func (job *RelocationJob) Run(context types.AsyncJobContext) {

	syncManager := context.GetSyncManager()
	ctx := context.GetGoContext()

	syncResult, err := syncManager.Sync(ctx, job.application)
	healthStatus := syncResult.Status

	if err != nil {
		job.Fail(context, err.Error())
		return
	} else {
		job.Success(context, healthStatus)
	}
}

func (job *RelocationJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.RelocationStatusFailure
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) Success(context types.AsyncJobContext, status *health.HealthStatus) {
	job.status = v1.RelocationStatusDone
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *RelocationJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeRelocate
}

func (job *RelocationJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *RelocationJob) Stop() {}
