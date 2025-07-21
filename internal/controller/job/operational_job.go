package job

import (
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

type LocalOperationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
	status        health.HealthStatusCode
	reason        string
	msg           string
	jobId         types.JobId
	log           logr.Logger
	events        *events.Events
	version       string
}

func NewLocalOperationJob(
	application *v1.AnyApplication,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *LocalOperationJob {
	log = log.WithName("LocalOperationJob")
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	version := application.ResourceVersion

	return &LocalOperationJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		status:        health.HealthStatusProgressing,
		jobId:         jobId,
		log:           log,
		version:       version,
		events:        events,
	}
}

func (job *LocalOperationJob) Run(context types.AsyncJobContext) {
	ticker := time.NewTicker(job.runtimeConfig.PollOperationalStatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			isCompleted := job.runInner(context)
			if isCompleted {
				return
			}
		case <-context.GetGoContext().Done():
			return
		}
	}
}

func (job *LocalOperationJob) runInner(context types.AsyncJobContext) bool {
	syncManager := context.GetSyncManager()

	healthStatus := syncManager.GetAggregatedStatus(job.application)

	job.status = healthStatus.Status
	job.msg = healthStatus.Message

	switch healthStatus.Status {
	case health.HealthStatusHealthy, health.HealthStatusProgressing:
		job.Success(context, healthStatus.Status)
		return false
	case health.HealthStatusDegraded, health.HealthStatusUnknown:
		job.Fail(context, healthStatus.Status, "", "HealthCheckFailed")
		return true

	case health.HealthStatusMissing:
		job.Fail(context, health.HealthStatusMissing, "Application resources are missing", "ResourcesMissing")
		return true
	default:
		return false
	}
}

func (job *LocalOperationJob) Fail(context types.AsyncJobContext, status health.HealthStatusCode, msg string, reason string) {

	job.msg = "Operation Failure: " + msg
	job.reason = reason
	job.status = status

	job.updateStatus(context)
}

func (job *LocalOperationJob) Success(context types.AsyncJobContext, status health.HealthStatusCode) {

	job.msg = "Operation state changed to '" + string(status) + "'. "
	job.reason = ""
	job.status = status

	job.updateStatus(context)
}

func (job *LocalOperationJob) updateStatus(jobContext types.AsyncJobContext) {
	statusUpdater := status.NewStatusUpdater(
		jobContext.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		jobContext.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.DeploymenConditionType, v1.UndeploymenConditionType)
	if err != nil {
		job.log.WithName("StatusUpdater").Error(err, "Failed to update status")
	}
}

func (job *LocalOperationJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *LocalOperationJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeLocalOperation
}

func (job *LocalOperationJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.LocalConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
		Reason:             job.reason,
	}
}
