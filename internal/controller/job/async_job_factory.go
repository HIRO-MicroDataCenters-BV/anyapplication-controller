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

type AsyncJobFactoryImpl struct {
	config *config.ApplicationRuntimeConfig
	clock  clock.Clock
	log    logr.Logger
	events *events.Events
}

func NewAsyncJobFactory(config *config.ApplicationRuntimeConfig, clock clock.Clock, log logr.Logger, events *events.Events) AsyncJobFactoryImpl {
	return AsyncJobFactoryImpl{
		config: config,
		clock:  clock,
		log:    log,
		events: events,
	}
}

func (f AsyncJobFactoryImpl) CreateDeployJob(application *v1.AnyApplication, version *types.SpecificVersion) types.AsyncJob {
	return NewDeployJob(application, version, f.config, f.clock, f.log, f.events)
}

func (f AsyncJobFactoryImpl) CreateOnwershipTransferJob(application *v1.AnyApplication) types.AsyncJob {
	return NewOwnershipTransferJob(application, f.config, f.clock, f.log, f.events)
}

func (f AsyncJobFactoryImpl) CreateUndeployJob(application *v1.AnyApplication) types.AsyncJob {
	return NewUndeployJob(application, f.config, f.clock, f.log, f.events)
}

func (f AsyncJobFactoryImpl) CreateLocalPlacementJob(application *v1.AnyApplication) types.AsyncJob {
	return NewLocalPlacementJob(application, f.config, f.clock, f.log, f.events)
}

func (f AsyncJobFactoryImpl) CreateOperationJob(application *v1.AnyApplication) types.AsyncJob {
	return NewLocalOperationJob(application, f.config, f.clock, f.log, f.events)
}
