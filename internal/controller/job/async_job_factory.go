package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
)

type AsyncJobFactoryImpl struct {
	config *config.ApplicationRuntimeConfig
}

func NewAsyncJobFactory(config *config.ApplicationRuntimeConfig) AsyncJobFactoryImpl {
	return AsyncJobFactoryImpl{
		config: config,
	}
}

func (f AsyncJobFactoryImpl) CreateRelocationJob(application *v1.AnyApplication) AsyncJob {
	return NewRelocationJob(application, f.config)
}

func (f AsyncJobFactoryImpl) CreateOnwershipTransferJob(application *v1.AnyApplication) AsyncJob {
	return NewOwnershipTransferJob(application, f.config)
}

func (f AsyncJobFactoryImpl) CreateUndeployJob(application *v1.AnyApplication) AsyncJob {
	return NewUndeployJob(application, f.config)
}
