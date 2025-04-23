package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type AsyncJobFactoryImpl struct {
	config *config.ApplicationRuntimeConfig
	clock  clock.Clock
}

func NewAsyncJobFactory(config *config.ApplicationRuntimeConfig, clock clock.Clock) AsyncJobFactoryImpl {
	return AsyncJobFactoryImpl{
		config: config,
		clock:  clock,
	}
}

func (f AsyncJobFactoryImpl) CreateRelocationJob(application *v1.AnyApplication) AsyncJob {
	return NewRelocationJob(application, f.config, f.clock)
}

func (f AsyncJobFactoryImpl) CreateOnwershipTransferJob(application *v1.AnyApplication) AsyncJob {
	return NewOwnershipTransferJob(application, f.config, f.clock)
}

func (f AsyncJobFactoryImpl) CreateUndeployJob(application *v1.AnyApplication) AsyncJob {
	return NewUndeployJob(application, f.config, f.clock)
}

func (f AsyncJobFactoryImpl) CreateLocalPlacementJob(application *v1.AnyApplication) AsyncJob {
	return NewLocalPlacementJob(application, f.config, f.clock)
}

func (f AsyncJobFactoryImpl) CreateOperationJob(application *v1.AnyApplication) AsyncJob {
	return NewLocalOperationJob(application, f.config, f.clock)
}
