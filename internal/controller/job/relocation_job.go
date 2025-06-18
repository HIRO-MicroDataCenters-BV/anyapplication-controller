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

type RelocationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
	jobId         types.JobId
	stopped       atomic.Bool
	log           logr.Logger
	version       string
	events        *events.Events
}

func NewRelocationJob(
	application *v1.AnyApplication,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *RelocationJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	version := application.ResourceVersion
	log = log.WithName("RelocationJob")
	return &RelocationJob{
		status:        v1.RelocationStatusPull,
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
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: "Relocation failure", Msg: job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, job.GetStatus(), event)
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) Success(context types.AsyncJobContext, healthStatus *health.HealthStatus) {
	job.status = v1.RelocationStatusDone

	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: "Relocation state changed to '" + string(job.status) + "'", Msg: job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, job.GetStatus(), event)
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
		ZoneVersion:        job.version,
	}
}

func (job *RelocationJob) Stop() {
	job.stopped.Store(true)
}
