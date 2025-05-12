package job

import (
	"sync/atomic"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/status"
	"hiro.io/anyapplication/internal/controller/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type UndeployJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
	jobId         types.JobId
	stopped       atomic.Bool
	log           logr.Logger
}

func NewUndeployJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *UndeployJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	log := logf.Log.WithName("UndeployJob")
	return &UndeployJob{
		status:        v1.RelocationStatusUndeploy,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
		jobId:         jobId,
		stopped:       atomic.Bool{},
		log:           log,
	}
}

func (job *UndeployJob) Run(context types.AsyncJobContext) {
	ctx := context.GetGoContext()

	syncManager := context.GetSyncManager()
	_, err := syncManager.Delete(ctx, job.application)

	if err != nil {
		job.Fail(context, err.Error())
		return
	} else {
		job.Success(context)
	}
}

func (job *UndeployJob) Fail(context types.AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.RelocationStatusFailure
	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(), job.log.WithName("UndeployJob StatusUpdater"), context.GetKubeClient(), job.application.GetNamespacedName())
	err := statusUpdater.UpdateCondition(&job.stopped, job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *UndeployJob) Success(context types.AsyncJobContext) {
	job.status = v1.RelocationStatusDone

	statusUpdater := status.NewStatusUpdater(
		context.GetGoContext(), job.log.WithName("UndeployJob StatusUpdater"), context.GetKubeClient(), job.application.GetNamespacedName())
	err := statusUpdater.UpdateCondition(&job.stopped, job.GetStatus())

	if err != nil {
		job.status = v1.RelocationStatusFailure
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
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *UndeployJob) Stop() {
	job.stopped.Store(true)
}
