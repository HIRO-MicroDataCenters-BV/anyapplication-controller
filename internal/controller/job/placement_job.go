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

type LocalPlacementJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	clock         clock.Clock
	status        v1.PlacementStatus
	msg           string
	jobId         types.JobId
	stopped       atomic.Bool
	log           logr.Logger
}

func NewLocalPlacementJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *LocalPlacementJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}
	log := logf.Log.WithName("LocalPlacementJob")
	return &LocalPlacementJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		status:        v1.PlacementStatusInProgress,
		jobId:         jobId,
		stopped:       atomic.Bool{},
		log:           log,
	}
}

func (job *LocalPlacementJob) Run(context types.AsyncJobContext) {
	client := context.GetKubeClient()
	ctx := context.GetGoContext()

	job.status = v1.PlacementStatusDone
	condition := job.GetStatus()

	statusUpdater := status.NewStatusUpdater(ctx, job.log.WithName("LocalPlacementJob StatusUpdater"), client, job.application.GetNamespacedName())
	err := statusUpdater.UpdateStatus(&job.stopped, func(applicationStatus *v1.AnyApplicationStatus) bool {
		applicationStatus.Placements = []v1.Placement{
			{
				Zone: job.runtimeConfig.ZoneId,
			},
		}
		status.AddOrUpdate(applicationStatus, &condition)
		return true
	})

	if err != nil {
		job.status = v1.PlacementStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *LocalPlacementJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *LocalPlacementJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeLocalPlacement
}

func (job *LocalPlacementJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.PlacementConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}

func (job *LocalPlacementJob) Stop() {
	job.stopped.Store(true)
}
