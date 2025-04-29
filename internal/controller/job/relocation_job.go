package job

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type RelocationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
}

func NewRelocationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *RelocationJob {
	return &RelocationJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
	}
}

func (job *RelocationJob) Run(context AsyncJobContext) {

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

func (job *RelocationJob) Fail(context AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.RelocationStatusFailure
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) Success(context AsyncJobContext, status *health.HealthStatus) {
	job.status = v1.RelocationStatusDone
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
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
		Msg:                job.msg,
	}
}
