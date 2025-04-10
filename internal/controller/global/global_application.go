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

func (g *GlobalApplication) DeriveNewStatus() mo.Option[v1.AnyApplicationStatus] {

	status := g.application.Status

	localConditionOpt := moutils.Map(g.locaApplication, func(l local.LocalApplication) v1.ConditionStatus {
		return l.GetCondition()
	})
	stateUpdated := moutils.Map(localConditionOpt, func(condition v1.ConditionStatus) bool {
		updateLocalCondition(&status, &condition, g.config)
		return true
	}).OrElse(false)

	stateUpdated = deriveState(&status, g.config) || stateUpdated

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

func deriveState(status *v1.AnyApplicationStatus, config *config.ApplicationRuntimeConfig) bool {
	if status.Owner != config.ZoneId {
		return false
	}
	newGlobalState := UnknownGlobal
	stateUpdated := false
	if status.State != string(newGlobalState) {
		status.State = string(newGlobalState)
		stateUpdated = true
	}
	return stateUpdated
}

// State      string            `json:"state,omitempty"`
// Placements []PlacementStatus `json:"placements,omitempty"`
// Owner      string            `json:"owner,omitempty"`
// Conditions []ConditionStatus `json:"conditions,omitempty"`
