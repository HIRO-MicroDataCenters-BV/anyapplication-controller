package job

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
)

type UndeployJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.UndeploymentStatus
	clock         clock.Clock
	msg           string
	reason        string
	jobId         types.JobId
	log           logr.Logger
	version       string
	events        *events.Events
	startTime     time.Time
	retryAttempts int
	attempt       int
}

func NewUndeployJob(
	application *v1.AnyApplication,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *UndeployJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeUndeploy,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	version := application.ResourceVersion
	log = log.WithName("UndeployJob")
	return &UndeployJob{
		status:        v1.UndeploymentStatusUndeploy,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
		log:           log,
		version:       version,
		events:        events,
		retryAttempts: 3,
		attempt:       1,
		startTime:     clock.NowTime().Time,
	}
}

func (job *UndeployJob) Run(jobContext types.AsyncJobContext) {
	isCompleted := job.runInner(jobContext)
	if isCompleted {
		return
	}

	ticker := time.NewTicker(job.runtimeConfig.PollSyncStatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			isCompleted := job.runInner(jobContext)
			if isCompleted {
				return
			}
		case <-jobContext.GetGoContext().Done():
			return
		}
	}
}

func (job *UndeployJob) runInner(jobContext types.AsyncJobContext) bool {

	applications := jobContext.GetApplications()
	versions, err := applications.GetAllPresentVersions(job.application)
	if err != nil {
		return job.maybeRetry(jobContext, "SyncError", fmt.Sprintf("Undeployment failed: %s", err.Error()))
	}
	if versions.IsEmpty() {
		job.Success(jobContext, "No versions found, undeployment is complete")
		return true
	}
	results, err := applications.Cleanup(jobContext.GetGoContext(), job.application)
	details := formatDeleteResultsMessage(results)
	if err != nil {
		return job.maybeRetry(jobContext, "SyncError", fmt.Sprintf("Undeployment failed: %s", err.Error()))
	} else {
		applicationResourcesPresent := types.IsApplicationResourcesPresent(results)

		if !applicationResourcesPresent {
			job.Success(jobContext, details)
			return true
		}
		if job.startTime.Add(job.runtimeConfig.DefaultUndeployTimeout).Before(job.clock.NowTime().Time) {
			return job.maybeRetry(jobContext, "Timeout", "Undeployment timed out")
		}
	}
	job.Progress(jobContext, details)
	return false
}

func formatDeleteResultsMessage(results []*types.DeleteResult) string {
	details := ""
	for _, result := range results {
		details += fmt.Sprintf(
			"Version %s (Total=%d, Deleted=%d, DeleteFailed=%d). ",
			result.Version.ToString(),
			result.Total,
			result.Deleted,
			result.DeleteFailed,
		)
	}
	return details
}

func (job *UndeployJob) maybeRetry(jobContext types.AsyncJobContext, reason string, failureMsg string) bool {
	if job.attempt < job.retryAttempts {
		job.attempt++
		job.startTime = job.clock.NowTime().Time
		job.AttemptFailure(
			jobContext,
			fmt.Sprintf("%s Retrying undeployment (attempt %v of %v).", failureMsg, job.attempt, job.retryAttempts),
			reason,
		)
		return false
	} else {
		job.Fail(
			jobContext,
			fmt.Sprintf("Failure after %v attempts.", job.retryAttempts),
			reason,
		)
		return true
	}
}

func (job *UndeployJob) AttemptFailure(jobContext types.AsyncJobContext, msg string, reason string) {

	job.status = v1.UndeploymentStatusUndeploy
	job.msg = "Undeploy failure: " + msg
	job.reason = reason

	job.updateStatus(jobContext)
}

func (job *UndeployJob) Fail(jobContext types.AsyncJobContext, msg string, reason string) {

	job.msg = "Undeploy failure: " + msg
	job.status = v1.UndeploymentStatusFailure
	job.reason = reason

	job.updateStatus(jobContext)
}

func (job *UndeployJob) Success(context types.AsyncJobContext, details string) {

	job.status = v1.UndeploymentStatusDone
	job.msg = "Undeploy state changed to '" + string(job.status) + "'." + details
	job.reason = ""

	job.updateStatus(context)
}

func (job *UndeployJob) Progress(context types.AsyncJobContext, details string) {
	job.msg = details
	job.reason = ""

	job.updateStatus(context)
}

func (job *UndeployJob) updateStatus(jobContext types.AsyncJobContext) {
	statusUpdater := status.NewStatusUpdater(
		jobContext.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		jobContext.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.LocalConditionType, v1.DeploymentConditionType)
	if err != nil {
		job.log.WithName("StatusUpdater").Error(err, "Failed to update status")
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
		Type:               v1.UndeploymentConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
		Reason:             job.reason,
	}
}
