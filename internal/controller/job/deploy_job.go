package job

import (
	"fmt"
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

type DeployJob struct {
	application   *v1.AnyApplication
	version       *types.SpecificVersion
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.DeploymentStatus
	clock         clock.Clock
	msg           string
	reason        string
	jobId         types.JobId
	log           logr.Logger
	events        *events.Events
	startTime     time.Time
	timeout       time.Duration
	retryAttempts int
	attempt       int
}

func NewDeployJob(
	application *v1.AnyApplication,
	version *types.SpecificVersion,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *DeployJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeDeploy,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	syncTimeout := config.GetSyncTimeout(application.Spec.SyncPolicy.SyncOptions, runtimeConfig.DefaultSyncTimeout)
	log = log.WithName("DeployJob")
	return &DeployJob{
		status:        v1.DeploymentStatusPull,
		application:   application,
		version:       version,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
		log:           log,
		events:        events,
		startTime:     clock.NowTime().Time,
		timeout:       syncTimeout,
		retryAttempts: 3,
		attempt:       1,
	}
}

func (job *DeployJob) Run(jobContext types.AsyncJobContext) {
	if job.runSyncCycle(jobContext) {
		return
	}

	ticker := time.NewTicker(job.runtimeConfig.PollSyncStatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			completed := job.runSyncCycle(jobContext)
			if completed {
				return
			}
		case <-jobContext.GetGoContext().Done():
			return
		}
	}

}
func (job *DeployJob) runSyncCycle(context types.AsyncJobContext) bool {
	applications := context.GetApplications()

	syncResult, err := applications.SyncVersion(context.GetGoContext(), job.application, job.version)
	healthStatus := syncResult.AggregatedStatus.HealthStatus

	if err != nil {
		job.Fail(context, err.Error(), "SyncError")
		return true
	}

	if syncResult.ApplicationResourcesDeployed {
		job.Success(context, healthStatus)
		return true
	}

	if job.startTime.Add(job.timeout).Before(job.clock.NowTime().Time) {
		if job.attempt < job.retryAttempts {
			job.attempt++
			job.startTime = job.clock.NowTime().Time
			job.log.Info("Retrying deployment", "attempt", job.attempt, "maxAttempts", job.retryAttempts)
			job.AttemptFailure(
				context,
				fmt.Sprintf("Retrying deployment (attempt %v of %v)", job.attempt, job.retryAttempts),
				"Timeout",
			)
		} else {
			job.Fail(
				context,
				"Deployment timed out after "+job.timeout.String(),
				"Timeout",
			)
			return true
		}
	}

	return false
}

func (job *DeployJob) AttemptFailure(jobContext types.AsyncJobContext, msg string, reason string) {
	job.status = v1.DeploymentStatusPull
	job.msg = "Deployment failure: " + msg
	job.reason = reason

	job.updateStatus(jobContext)
}

func (job *DeployJob) Fail(jobContext types.AsyncJobContext, msg string, reason string) {
	job.status = v1.DeploymentStatusFailure
	job.msg = "Deployment failure: " + msg
	job.reason = reason

	job.updateStatus(jobContext)
}

func (job *DeployJob) Success(jobContext types.AsyncJobContext, healthStatus *health.HealthStatus) {
	job.status = v1.DeploymentStatusDone
	job.msg = "Deployment state changed to '" + string(job.status) + "'. "
	job.reason = ""

	job.updateStatus(jobContext)
}

func (job *DeployJob) updateStatus(jobContext types.AsyncJobContext) {
	statusUpdater := status.NewStatusUpdater(
		jobContext.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		jobContext.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.UndeploymentConditionType, v1.LocalConditionType)
	if err != nil {
		job.log.WithName("StatusUpdater").Error(err, "Failed to update status")
	}
}

func (job *DeployJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *DeployJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeDeploy
}

func (job *DeployJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.DeploymentConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
		Reason:             job.reason,
	}
}
