package job

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/samber/mo"
	"hiro.io/anyapplication/internal/controller/types"
)

type CleanupFunc func(jobID types.JobId)

type jobs struct {
	jobs       sync.Map
	jobContext types.AsyncJobContext
	cancel     context.CancelFunc
}

func NewJobs(jobContext types.AsyncJobContext) *jobs {
	jobContext, cancel := jobContext.WithCancel()
	return &jobs{
		jobContext: jobContext,
		jobs:       sync.Map{},
		cancel:     cancel,
	}
}

func (j *jobs) StopAll() {
	j.cancel()
}

func (j *jobs) Execute(job types.AsyncJob) {
	jobId := job.GetJobID()
	id := jobId.ApplicationId
	worker := NewJobWorker(job, func(jobID types.JobId) {
		worker, found := j.jobs.Load(jobID.ApplicationId)
		if !found {
			return
		}
		jobWorker, ok := worker.(*JobWorker)
		if ok && jobWorker.getJob().GetJobID() == jobID {
			j.jobs.Delete(jobID.ApplicationId)
		}
	})
	j.jobs.Store(id, worker)
	go worker.Run(j.jobContext)
}

func (j *jobs) GetCurrent(id types.ApplicationId) mo.Option[types.AsyncJob] {
	worker, found := j.jobs.Load(id)
	if !found {
		return mo.None[types.AsyncJob]()
	}
	jobWorker, ok := worker.(*JobWorker)
	if !ok {
		panic("Unexpected type")
	}
	return mo.Some(jobWorker.getJob())
}

func (j *jobs) Stop(id types.ApplicationId) {
	worker, found := j.jobs.LoadAndDelete(id)
	if !found {
		return
	}
	jobWorker, ok := worker.(*JobWorker)
	if !ok {
		panic("Unexpected type")
	}
	jobWorker.Stop()
}

type JobWorker struct {
	job         types.AsyncJob
	stopped     atomic.Bool
	cancelFunc  *context.CancelFunc
	cleanupFunc CleanupFunc
}

func NewJobWorker(job types.AsyncJob, cleanupFunc CleanupFunc) *JobWorker {
	return &JobWorker{
		job:         job,
		cleanupFunc: cleanupFunc,
		stopped:     atomic.Bool{},
	}
}

func (w *JobWorker) Run(jobContext types.AsyncJobContext) {
	contextWithCancel, cancel := jobContext.WithCancel()
	w.cancelFunc = &cancel
	w.job.Run(contextWithCancel)
	w.Stop()
}

func (w *JobWorker) getJob() types.AsyncJob {
	return w.job
}

func (w *JobWorker) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		if w.cancelFunc != nil {
			(*w.cancelFunc)()
		}
		w.cleanupFunc(w.job.GetJobID())
	}
}
