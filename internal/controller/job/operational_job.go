package job

import (
	"sync/atomic"
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
	application        *v1.AnyApplication
	runtimeConfig      *config.ApplicationRuntimeConfig
	clock              clock.Clock
	status             health.HealthStatusCode
	msg                string
	stopCh             chan struct{}
	stopConfirmChannel chan struct{}
	jobId              types.JobId
	stopped            atomic.Bool
	log                logr.Logger
	events             *events.Events
	version            string
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
		application:        application,
		runtimeConfig:      runtimeConfig,
		clock:              clock,
		status:             health.HealthStatusProgressing,
		stopCh:             make(chan struct{}),
		stopConfirmChannel: make(chan struct{}),
		jobId:              jobId,
		stopped:            atomic.Bool{},
		log:                log,
		version:            version,
		events:             events,
	}
}

func (job *LocalOperationJob) Run(context types.AsyncJobContext) {
	go func() {
		defer close(job.stopConfirmChannel)

		job.runInner(context)

		ticker := time.NewTicker(job.runtimeConfig.PollOperationalStatusInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				job.runInner(context)
			case <-job.stopCh:
				return
			}
		}
	}()
}

func (job *LocalOperationJob) runInner(context types.AsyncJobContext) {
	syncManager := context.GetSyncManager()

	healthStatus := syncManager.GetAggregatedStatus(job.application)
	job.status = healthStatus.Status
	job.msg = healthStatus.Message

	switch healthStatus.Status {
	case health.HealthStatusHealthy, health.HealthStatusProgressing:
		job.Success(context)
	case health.HealthStatusDegraded, health.HealthStatusUnknown:
		job.Fail(context)
	case health.HealthStatusMissing:
		syncResult, err := syncManager.Sync(context.GetGoContext(), job.application)
		if err != nil {
			job.status = health.HealthStatusDegraded
			job.msg = "Cannot sync application: " + err.Error()
			job.Fail(context)
		} else {
			job.status = syncResult.Status.Status
			job.msg = syncResult.Status.Message
			job.Success(context)
		}
	}
}

func (job *LocalOperationJob) Stop() {
	job.stopped.Store(true)
	job.stopCh <- struct{}{}
	<-job.stopConfirmChannel
}

func (job *LocalOperationJob) Fail(context types.AsyncJobContext) {
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Operation Failure: " + job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.DeploymenConditionType, v1.UndeploymenConditionType)
	if err != nil {
		job.status = health.HealthStatusDegraded
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalOperationJob) Success(context types.AsyncJobContext) {
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Operation state change to " + string(job.status) + job.msg}
	err := statusUpdater.UpdateCondition(event, job.GetStatus(), v1.DeploymenConditionType, v1.UndeploymenConditionType)
	if err != nil {
		job.status = health.HealthStatusDegraded
		job.msg = "Cannot Update Application Condition. " + err.Error()
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
	}
}
