package job

import (
	"sync"

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
	j.jobs.Store(id, job)
	go job.Run(j.context)
}

func (j *jobs) GetCurrent(id types.ApplicationId) mo.Option[types.AsyncJob] {
	job, found := j.jobs.Load(id)
	if !found {
		return mo.None[types.AsyncJob]()
	}
	asyncJob, ok := job.(types.AsyncJob)
	if !ok {
		panic("Unexpected type")
	}
	return mo.Some(asyncJob)
}

func (j *jobs) Stop(id types.ApplicationId) {
	job, found := j.jobs.LoadAndDelete(id)
	if !found {
		return
	}
	asyncJob, ok := job.(types.AsyncJob)
	if !ok {
		panic("Unexpected type")
	}
	asyncJob.Stop()
}
