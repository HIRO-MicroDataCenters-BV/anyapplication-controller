package job

import (
	"fmt"
	"time"
)

type JobExecutorActor struct {
	ID      int
	context AsyncJobContext
	JobChan chan AsyncJob
	Quit    chan bool
	Current chan AsyncJob
}

func NewActor(id int, context AsyncJobContext) *JobExecutorActor {
	return &JobExecutorActor{
		ID:      id,
		JobChan: make(chan AsyncJob),
		Quit:    make(chan bool),
		context: context,
	}
}

func (a *JobExecutorActor) Start() {
	go func() {
		for {
			select {
			case job := <-a.JobChan:
				// Simulate processing
				time.Sleep(500 * time.Millisecond)
				_ = fmt.Sprintf("Actor %d processed job", a.ID)
				job.Run(a.context)

			case <-a.Quit:
				fmt.Printf("Actor %d quitting\n", a.ID)
				return
			}
		}
	}()
}

func (a *JobExecutorActor) GetCurrent() {
	a.Quit <- true
}

func (a *JobExecutorActor) Stop() {
	a.Quit <- true
}
