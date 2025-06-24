package job

import (
	"sync/atomic"

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
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.DeploymentStatus
	clock         clock.Clock
	msg           string
	jobId         types.JobId
	stopped       atomic.Bool
	log           logr.Logger
	version       string
	events        *events.Events
}

func NewDeployJob(
	application *v1.AnyApplication,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *DeployJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	version := application.ResourceVersion
	log = log.WithName("DeployJob")
	return &DeployJob{
		status:        v1.DeploymentStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
		stopped:       atomic.Bool{},
		log:           log,
		version:       version,
		events:        events,
	}
}

func (job *DeployJob) Run(context types.AsyncJobContext) {

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

func (job *DeployJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.DeploymentStatusFailure
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Deployment failure: " + job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, event, job.GetStatus(), v1.UndeploymenConditionType, v1.LocalConditionType)
	if err != nil {
		job.status = v1.DeploymentStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *DeployJob) Success(context types.AsyncJobContext, healthStatus *health.HealthStatus) {
	job.status = v1.DeploymentStatusDone

	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Deployment state changed to '" + string(job.status) + "'. " + job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, event, job.GetStatus(), v1.UndeploymenConditionType, v1.LocalConditionType)
	if err != nil {
		job.status = v1.DeploymentStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
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
		Type:               v1.DeploymenConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *DeployJob) Stop() {
	job.stopped.Store(true)
}
