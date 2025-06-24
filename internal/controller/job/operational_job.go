package job

import (
	"sync"
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
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
	status        health.HealthStatusCode
	msg           string
	stopCh        chan struct{}
	wg            sync.WaitGroup
	jobId         types.JobId
	stopped       atomic.Bool
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
		stopCh:        make(chan struct{}),
		jobId:         jobId,
		stopped:       atomic.Bool{},
		log:           log,
		version:       version,
		events:        events,
	}
}

func (job *LocalOperationJob) Run(context types.AsyncJobContext) {
	job.wg.Add(1)

	go func() {
		defer job.wg.Done()

		job.runInner(context)

		ticker := time.NewTicker(job.runtimeConfig.LocalPollInterval)
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

	syncResult, err := syncManager.Sync(context.GetGoContext(), job.application)
	healthStatus := syncResult.Status

	if err != nil {
		job.Fail(context, err.Error())
		return
	} else {
		job.Success(context, healthStatus)
	}
}

func (job *LocalOperationJob) Stop() {
	job.stopped.Store(true)
	close(job.stopCh)
	job.wg.Wait()
}

func (job *LocalOperationJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = health.HealthStatusDegraded

	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Operation Failure: " + job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, event, job.GetStatus(), v1.DeploymenConditionType, v1.UndeploymenConditionType)
	if err != nil {
		job.status = health.HealthStatusDegraded
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalOperationJob) Success(context types.AsyncJobContext, healthStatus *health.HealthStatus) {
	job.status = healthStatus.Status
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(),
		job.log.WithName("StatusUpdater"),
		context.GetKubeClient(),
		job.application.GetNamespacedName(),
		job.runtimeConfig.ZoneId,
		job.events,
	)
	event := events.Event{Reason: events.LocalStateChangeReason, Msg: "Operation state change to " + string(job.status) + job.msg}
	err := statusUpdater.UpdateCondition(&job.stopped, event, job.GetStatus(), v1.DeploymenConditionType, v1.UndeploymenConditionType)
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
