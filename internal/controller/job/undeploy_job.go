package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UndeployJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
}

func NewUndeployJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig) UndeployJob {
	return UndeployJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
	}
}

func (job UndeployJob) Run() {

}

func (job UndeployJob) GetJobID() JobId {
	return JobId{}
}

func (job UndeployJob) GetType() AsyncJobType {
	return AsyncJobTypeUndeploy
}

func (job UndeployJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: metav1.Now(),
	}
}
