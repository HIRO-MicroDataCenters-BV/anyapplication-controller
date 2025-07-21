package status

import (
	"context"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/events"
	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusUpdater struct {
	ctx    context.Context
	log    logr.Logger
	client client.Client
	name   types.NamespacedName
	zoneId string
	events *events.Events
}

func NewStatusUpdater(
	ctx context.Context,
	log logr.Logger,
	client client.Client,
	name types.NamespacedName,
	zoneId string,
	events *events.Events,
) StatusUpdater {
	return StatusUpdater{
		ctx:    ctx,
		log:    log,
		client: client,
		name:   name,
		zoneId: zoneId,
		events: events,
	}
}

func (su *StatusUpdater) UpdateStatus(
	statusUpdate func(status *v1.AnyApplicationStatus, zoneId string) (bool, events.Event),
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var application v1.AnyApplication
		if err := su.client.Get(su.ctx, su.name, &application); err != nil {
			if errors.IsNotFound(err) {
				return nil // Resource gone
			}
			return err
		}

		updatedApplication := application.DeepCopy()
		updated := false
		eventToSend := events.Event{}
		if statusUpdate != nil {
			updated, eventToSend = statusUpdate(&updatedApplication.Status, su.zoneId)
		}
		// Update or insert condition
		if updated {
			if ctrltypes.IsCancelled(su.ctx) {
				return nil // stop retrying
			}
			updatedApplication.IncrementZoneVersion(su.zoneId)

			err := su.client.Status().Update(su.ctx, updatedApplication)
			if err == nil {
				su.events.Emit(updatedApplication, eventToSend)
				updatedApplication.Status.LogStatus()
				su.log.Info("Updating status", "status", updatedApplication.Status, "error", err)
			}
			return err
		}
		return nil // no change needed
	})
}

func (su *StatusUpdater) UpdateCondition(
	event events.Event,
	conditionToUpdate v1.ConditionStatus,
	conditionsToRemove ...v1.ApplicationConditionType,
) error {
	return su.UpdateStatus(func(status *v1.AnyApplicationStatus, zoneId string) (bool, events.Event) {
		updated := status.AddOrUpdate(&conditionToUpdate, zoneId)
		for _, condType := range conditionsToRemove {
			removed := status.Remove(condType, zoneId)
			updated = updated || removed
		}
		return updated, event
	})
}
