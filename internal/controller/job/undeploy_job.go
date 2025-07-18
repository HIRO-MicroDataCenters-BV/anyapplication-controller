package job

import (
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
	jobId         types.JobId
	log           logr.Logger
	version       string
	events        *events.Events
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
	}
}

func (job *UndeployJob) Run(context types.AsyncJobContext) {
	job.runInner(context)
}

func (job *UndeployJob) runInner(context types.AsyncJobContext) {
	ctx := context.GetGoContext()

	syncManager := context.GetSyncManager()
	_, err := syncManager.Delete(ctx, job.application)

	if err != nil {
		job.Fail(context, err.Error())
	} else {
		job.Success(context)
	}
}

func (job *UndeployJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.UndeploymentStatusFailure
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Undeploy failure: " + job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.LocalConditionType, v1.DeploymenConditionType)
	if err != nil {
		job.status = v1.UndeploymentStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *UndeployJob) Success(context types.AsyncJobContext) {
	job.status = v1.UndeploymentStatusDone

	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Undeploy state changed to '" + string(job.status) + "'" + job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.LocalConditionType, v1.DeploymenConditionType)

	if err != nil {
		job.status = v1.UndeploymentStatusFailure
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
		Type:               v1.UndeploymenConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}
