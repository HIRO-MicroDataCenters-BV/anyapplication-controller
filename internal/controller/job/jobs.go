package job

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/samber/mo"
	"hiro.io/anyapplication/internal/controller/types"
)

type jobs struct {
	jobs    sync.Map
	context types.AsyncJobContext
}

func NewJobs(context types.AsyncJobContext) *jobs {
	return &jobs{
		context: context,
		jobs:    sync.Map{},
	}
}

func (j *jobs) StopAll() {
	panic("not implemented")
}

func (j *jobs) Execute(job types.AsyncJob) {
	jobId := job.GetJobID()
	id := jobId.ApplicationId
	worker := NewJobWorker(job)
	j.jobs.Store(id, worker)
	go worker.Run(j.context)
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
	job                types.AsyncJob
	stopped            atomic.Bool
	stopConfirmChannel chan struct{}
	cancelFunc         *context.CancelFunc
}

func NewJobWorker(job types.AsyncJob) *JobWorker {
	return &JobWorker{
		job:                job,
		stopped:            atomic.Bool{},
		stopConfirmChannel: make(chan struct{}),
	}
}

func (w *JobWorker) Run(jobContext types.AsyncJobContext) {
	jobContext, cancel := jobContext.WithCancel()
	w.cancelFunc = &cancel
	w.job.Run(jobContext)
	close(w.stopConfirmChannel)
}

func (w *JobWorker) getJob() types.AsyncJob {
	return w.job
}

func (w *JobWorker) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		if w.cancelFunc != nil {
			(*w.cancelFunc)()
		}
		<-w.stopConfirmChannel
	}
}
