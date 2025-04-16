package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OwnershipTransferJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.OwnershipTransferStatus
}

func NewOwnershipTransferJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig) OwnershipTransferJob {
	return OwnershipTransferJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		status:        v1.OwnershipTransferPulling,
	}
}

func (job OwnershipTransferJob) Run(context AsyncJobContext) {

}

func (job OwnershipTransferJob) GetJobID() JobId {
	return JobId{}
}

func (job OwnershipTransferJob) GetType() AsyncJobType {
	return AsyncJobTypeOwnershipTransfer
}

func (job OwnershipTransferJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.OwnershipTransferConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: metav1.Now(),
	}
}
