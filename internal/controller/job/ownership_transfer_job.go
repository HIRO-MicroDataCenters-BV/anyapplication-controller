// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package job

import (
	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/events"
	"hiro.io/anyapplication/internal/controller/types"
)

type OwnershipTransferJob struct {
	application   *v1.AnyApplication
	runtimeConfig *config.ApplicationRuntimeConfig
	status        v1.OwnershipTransferStatus
	clock         clock.Clock
	jobId         types.JobId
	log           logr.Logger
	events        *events.Events
	reason        string
}

func NewOwnershipTransferJob(
	application *v1.AnyApplication,
	runtimeConfig *config.ApplicationRuntimeConfig,
	clock clock.Clock,
	log logr.Logger,
	events *events.Events,
) *OwnershipTransferJob {
	jobId := types.JobId{
		JobType: types.AsyncJobTypeLocalOperation,
		ApplicationId: types.ApplicationId{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	return &OwnershipTransferJob{
		application:   application,
		runtimeConfig: runtimeConfig,
		status:        v1.OwnershipTransferPulling,
		clock:         clock,
		jobId:         jobId,
		log:           log,
		events:        events,
	}
}

func (job *OwnershipTransferJob) Run(context types.AsyncJobContext) {

}

func (job *OwnershipTransferJob) GetJobID() types.JobId {
	return job.jobId
}

func (job *OwnershipTransferJob) GetType() types.AsyncJobType {
	return types.AsyncJobTypeOwnershipTransfer
}

func (job *OwnershipTransferJob) GetStatus() v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               v1.OwnershipTransferConditionType,
		ZoneId:             job.runtimeConfig.ZoneId,
		Status:             string(job.status),
		LastTransitionTime: job.clock.NowTime(),
		Msg:                "Ownership transfer in progress",
		Reason:             job.reason,
	}
}

func (job *OwnershipTransferJob) Stop() {}
