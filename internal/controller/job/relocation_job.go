package job

import (
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
)

type RelocationJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.RelocationStatus
	clock         clock.Clock
	msg           string
}

func NewRelocationJob(application *v1.AnyApplication, runtimeConfig *config.ApplicationRuntimeConfig, clock clock.Clock) *RelocationJob {
	return &RelocationJob{
		status:        v1.RelocationStatusPull,
		application:   application,
		runtimeConfig: runtimeConfig,
		clock:         clock,
		msg:           "",
	}
}

func (job *RelocationJob) Run(context AsyncJobContext) {
	// client := context.GetKubeClient()
	// ctx := context.GetGoContext()
	// helmClient := context.GetHelmClient()

	// releaseName := job.application.Name
	// helmSelector := job.application.Spec.Application.HelmSelector
	// labels := map[string]string{"dcp.hiro.io/managed-by": "dcp"}
	// values := values.Options{}
	// template, err := helmClient.Template(&helm.TemplateArgs{
	// 	ReleaseName:   releaseName,
	// 	RepoUrl:       helmSelector.Repository,
	// 	ChartName:     helmSelector.Chart,
	// 	Namespace:     helmSelector.Namespace,
	// 	Version:       helmSelector.Version,
	// 	ValuesOptions: values,
	// 	Labels:        labels,
	// })
	// if err != nil {
	// 	job.Fail(context, err.Error())
	// 	return
	// }

	// localApplicationOpt, err := local.NewLocalApplicationFromTemplate(template)
	// if err != nil {
	// 	job.Fail(context, err.Error())
	// 	return
	// }

	// if localApplicationOpt.IsPresent() {

	// }
	job.Success(context)
}

func (job *RelocationJob) Fail(context AsyncJobContext, msg string) {
	job.msg = msg
	job.status = v1.RelocationStatusFailure
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) Success(context AsyncJobContext) {
	job.status = v1.RelocationStatusDone
	err := AddOrUpdateStatusCondition(context.GetGoContext(), context.GetKubeClient(), job.application.GetNamespacedName(), job.GetStatus())
	if err != nil {
		job.status = v1.RelocationStatusFailure
		job.msg = "Cannot Update Application Condition. " + err.Error()
	}
}

func (job *RelocationJob) GetJobID() JobId {
	return JobId{}
}

func (job *RelocationJob) GetType() AsyncJobType {
	return AsyncJobTypeRelocate
}

func (job *RelocationJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.RelocationConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                job.msg,
	}
}
