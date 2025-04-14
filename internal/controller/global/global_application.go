package global

import (
	"context"

	"github.com/samber/lo"
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/config"
	"hiro.io/anyapplication/internal/controller/local"
	"hiro.io/anyapplication/internal/moutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Action string

const (
	RelocateToCurrentNode Action = "RelocateToCurrentNode"
)

type GlobalApplication struct {
	locaApplication mo.Option[local.LocalApplication]
	application     *v1.AnyApplication
	config          *config.ApplicationRuntimeConfig
}

func LoadCurrentState(ctx context.Context, client client.Client, application *v1.AnyApplication, config *config.ApplicationRuntimeConfig) GlobalApplication {
	localApplication, err := local.LoadCurrentState(ctx, client, &application.Spec.Application, config)
	if err != nil {
		log.Log.Info("error loading current state")
	}
	return GlobalApplication{
		locaApplication: localApplication,
		application:     application,
		config:          config,
	}
}

func (g *GlobalApplication) DeriveNewStatus(jobConditions JobApplicationConditions) mo.Option[v1.AnyApplicationStatus] {

	status := g.application.Status
	// Update local application status if exists
	localConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
		return l.GetCondition()
	})
	stateUpdated := moutils.Map(localConditionOpt, func(condition v1.ConditionStatus) bool {
		updateLocalCondition(&status, &condition, g.config)
		return true
	}).OrElse(false)

	// Update job conditions
	stateUpdated = updateJobConditions(status, jobConditions) || stateUpdated

	// Update global state
	globalStateUpdated := updateGlobalState(&status, g.config)
	stateUpdated = globalStateUpdated || stateUpdated

	if stateUpdated {
		return mo.Some(status)
	} else {
		return mo.None[v1.AnyApplicationStatus]()
	}
}

func updateLocalCondition(status *v1.AnyApplicationStatus, condition *v1.ConditionStatus, config *config.ApplicationRuntimeConfig) {
	found, ok := lo.Find(status.Conditions, func(cond v1.ConditionStatus) bool {
		return cond.ZoneId == config.ZoneId
	})
	if !ok {
		status.Conditions = append(status.Conditions, *condition)
	} else {
		condition.DeepCopyInto(&found)
	}
}

func updateGlobalState(status *v1.AnyApplicationStatus, config *config.ApplicationRuntimeConfig) bool {
	if status.Owner != config.ZoneId {
		return false
	}
	stateUpdated := false
	newGlobalState := status.State
	if status.State == v1.NewGlobalState {
		// if current state is new and current node is owner
		newGlobalState = v1.PlacementGlobalState
		stateUpdated = true
	} else if status.State == v1.PlacementGlobalState {
		// if current state is placements and current node is owner
		// check if placements are set
		if !currentNodeInPlacementList(status, config.ZoneId) {
			newGlobalState = v1.OwnershipTransferGlobalState
			stateUpdated = true
		}
		// Do nothing and expect placement controller to set the placements
		//
	} else if status.State == v1.OperationalGlobalState {
		if !currentNodeInPlacementList(status, config.ZoneId) {
			newGlobalState = v1.OwnershipTransferGlobalState
			stateUpdated = true
		} else if isFailureCondition(status, config) {
			newGlobalState = v1.FailureGlobalState
			stateUpdated = true
		} else if currentNodeInPlacementList(status, config.ZoneId) && currentNodeNotInConditions(status, config.ZoneId) {
			newGlobalState = v1.RelocationGlobalState
			stateUpdated = true
		}
	} else if status.State == v1.FailureGlobalState {
		// Expect input from placement controller

	} else if status.State == v1.RelocationGlobalState {
		// set by owning controller

		if !actionInConditionList(status, v1.ApplicationConditionType(RelocateToCurrentNode)) {

		}

	} else if status.State == v1.OwnershipTransferGlobalState {
		if !currentNodeInPlacementList(status, config.ZoneId) {
			// Do nothing still in the transfer state
		}

	}
	// If the owner is current node and local application is not running
	// Set ownership transfer state

	// If the owner is current node and local application is not running
	// Set ownership transfer state

	// If the conditions and placements are not syncronized
	// Set relocation state

	if status.State != newGlobalState {
		status.State = newGlobalState
		stateUpdated = true
	}
	return stateUpdated
}

func currentNodeInPlacementList(status *v1.AnyApplicationStatus, currentZone string) bool {
	if status.Placements == nil {
		return false
	}
	_, ok := lo.Find(status.Placements, func(placement v1.Placement) bool {
		return placement.Zone == currentZone
	})
	return ok
}

func isFailureCondition(status *v1.AnyApplicationStatus, config *config.ApplicationRuntimeConfig) bool {
	return false
}

func updateJobConditions(status v1.AnyApplicationStatus, jobConditions JobApplicationConditions) bool {

	// localConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
	// 	return l.GetCondition()
	// })
	// stateUpdated := moutils.Map(localConditionOpt, func(condition v1.ConditionStatus) bool {
	// 	updateLocalCondition(&status, &condition, g.config)
	// 	return true
	// }).OrElse(false)

	return false
}

func currentNodeNotInConditions(status *v1.AnyApplicationStatus, currentZone string) bool {
	return false
}

func actionInConditionList(status *v1.AnyApplicationStatus, action v1.ApplicationConditionType) bool {
	_, ok := lo.Find(status.Conditions, func(condition v1.ConditionStatus) bool {
		return condition.Type == action
	})
	return ok
}

// func getCurrentActionStatus(status *v1.AnyApplicationStatus, action Action) mo.Option[v1.ConditionStatus] {
// 	found, ok := lo.Find(status.Conditions, func(condition v1.ConditionStatus) bool {
// 		return condition.Type == action
// 	})
// 	return ok
// }
