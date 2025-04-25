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
	job.Success(context)
}

func (job *LocalOperationJob) Fail(context AsyncJobContext, msg string) {
	job.msg = msg
	job.status = health.HealthStatusDegraded
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = health.HealthStatusDegraded
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalOperationJob) Success(context AsyncJobContext) {
	job.status = health.HealthStatusHealthy
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = health.HealthStatusHealthy
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
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
