package job

type Dispatcher struct {
	Actors []*JobExecutorActor
}

func NewDispatcher(numActors int) *Dispatcher {
	actors := make([]*JobExecutorActor, numActors)
	for i := 0; i < numActors; i++ {
		actor := NewActor(i + 1)
		actor.Start()
		actors[i] = actor
	}
	return &Dispatcher{Actors: actors}
}

func (d *Dispatcher) SubmitJob(job AsyncJob) {
	// Round-robin for simplicity
	// actor := d.Actors[job.GetJobID()%len(d.Actors)]
	// actor.JobChan <- job
}

func (d *Dispatcher) StopAll() {
	for _, actor := range d.Actors {
		actor.Stop()
	}
}
