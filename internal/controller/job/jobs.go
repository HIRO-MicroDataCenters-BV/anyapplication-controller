package job

import (
	"sync"

	"github.com/samber/mo"
)

type jobs struct {
	jobs    sync.Map
	context AsyncJobContext
}

func NewJobs(context AsyncJobContext) *jobs {
	return &jobs{
		context: context,
		jobs:    sync.Map{},
	}
}

func (j *jobs) StopAll() {

}

func (j *jobs) Execute(job AsyncJob) {
	jobId := job.GetJobID()
	id := jobId.ApplicationId
	j.jobs.Store(id, job)
	go job.Run(j.context)
}

func (j *jobs) GetCurrent(id ApplicationId) mo.Option[AsyncJob] {
	job, found := j.jobs.Load(id)
	if !found {
		return mo.None[AsyncJob]()
	}
	asyncJob, ok := job.(AsyncJob)
	if !ok {
		panic("Unexpected type")
	}
	return mo.Some(asyncJob)
}

func (j *jobs) Stop(id ApplicationId) {
	job, found := j.jobs.Load(id)
	if !found {
		return
	}
	asyncJob, ok := job.(AsyncJob)
	if !ok {
		panic("Unexpected type")
	}
	asyncJob.Stop()
}
