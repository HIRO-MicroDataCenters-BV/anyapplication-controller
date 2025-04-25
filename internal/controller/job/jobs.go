package job

import (
	"github.com/samber/mo"
	"k8s.io/apimachinery/pkg/types"
)

type jobs struct {
	// Actors []*JobExecutorActor
}

func NewJobs() *jobs {
	// actors := make([]*JobExecutorActor, numActors)
	// for i := 0; i < numActors; i++ {
	// 	actor := NewActor(i + 1)
	// 	actor.Start()
	// 	actors[i] = actor
	// }
	// return &jobs{Actors: actors}
	return &jobs{}
}

func (d *jobs) StopAll() {
	// for _, actor := range d.Actors {
	// 	actor.Stop()
	// }
}

func (d *jobs) Execute(job AsyncJob) {

}

func (d *jobs) GetCurrent(name types.NamespacedName) mo.Option[AsyncJob] {
	return mo.None[AsyncJob]()
}

func (d *jobs) Stop(job AsyncJob) {
}
