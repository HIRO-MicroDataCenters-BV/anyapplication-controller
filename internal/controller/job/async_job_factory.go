package job

import v1 "hiro.io/anyapplication/api/v1"

type AsyncJobFactoryImpl struct {
}

func (f AsyncJobFactoryImpl) CreateRelocationJob(application *v1.AnyApplication) *AsyncJob {
	panic("implement me")
}
func (f AsyncJobFactoryImpl) CreateOnwershipTransferJob(application *v1.AnyApplication) *AsyncJob {
	panic("implement me")
}
