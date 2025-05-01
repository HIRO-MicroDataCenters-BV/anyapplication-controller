package job

import (
	"sync"
	"time"

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
	stopCh        chan struct{}
	wg            sync.WaitGroup
	jobId         JobId
}

func NewLocalOperationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *LocalOperationJob {
	jobId := JobId{
		JobType: AsyncJobTypeLocalOperation,
		ApplicationId: ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &LocalOperationJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		status:        health.HealthStatusProgressing,
		stopCh:        make(chan struct{}),
		jobId:         jobId,
	}
}

func (job *LocalOperationJob) Run(context AsyncJobContext) {
	job.wg.Add(1)

	go func() {
		defer job.wg.Done()

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

func (job *LocalOperationJob) runInner(context AsyncJobContext) {
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
	close(job.stopCh)
	job.wg.Wait()
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

func (job *LocalOperationJob) Success(context AsyncJobContext, status *health.HealthStatus) {
	job.status = status.Status
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = health.HealthStatusDegraded
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalOperationJob) GetJobID() JobId {
	return job.jobId
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
