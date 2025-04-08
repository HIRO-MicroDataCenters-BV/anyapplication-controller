package job

import (
	"fmt"
	"time"
)

type JobExecutorActor struct {
	ID      int
	JobChan chan AsyncJob
	Quit    chan bool
}

func NewActor(id int) *JobExecutorActor {
	return &JobExecutorActor{
		ID:      id,
		JobChan: make(chan AsyncJob),
		Quit:    make(chan bool),
	}
}

func (a *JobExecutorActor) Start() {
	go func() {
		for {
			select {
			case job := <-a.JobChan:
				// Simulate processing
				time.Sleep(500 * time.Millisecond)
				_ = fmt.Sprintf("Actor %d processed job %d", a.ID, job.GetJobID())
				// result := fmt.Sprintf("Actor %d processed job %d with payload: %s", a.ID, job.GetJobID(), job.Payload)
				// job.Result <- result

			case <-a.Quit:
				fmt.Printf("Actor %d quitting\n", a.ID)
				return
			}
		}
	}()
}

func (a *JobExecutorActor) Stop() {
	a.Quit <- true
}
