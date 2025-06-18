package status

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller/events"
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
	stopRetrying *atomic.Bool,
	statusUpdate func(status *v1.AnyApplicationStatus) (bool, events.Event),
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
			updated, eventToSend = statusUpdate(&updatedApplication.Status)
		}
		// Update or insert condition
		if updated {
			if stopRetrying != nil && stopRetrying.Load() {
				return nil // stop retrying
			}
			incrementZoneVersion(updatedApplication, su.zoneId)
			err := su.client.Status().Update(su.ctx, updatedApplication)
			if err == nil {
				su.events.Emit(updatedApplication, eventToSend)
			}
			su.log.Info("Updating status", "status", updatedApplication.Status, "error", err)
			return err
		}
		return nil // no change needed
	})
}

func (su *StatusUpdater) UpdateCondition(
	stopRetrying *atomic.Bool,
	conditionToUpdate v1.ConditionStatus,
	event events.Event,
) error {
	return su.UpdateStatus(stopRetrying, func(status *v1.AnyApplicationStatus) (bool, events.Event) {
		updated := AddOrUpdate(status, &conditionToUpdate)
		return updated, event
	})
}

func AddOrUpdate(status *v1.AnyApplicationStatus, toAddOrUpdate *v1.ConditionStatus) bool {
	existing := status.Conditions
	updated := false
	found := false

	for i, cond := range existing {
		if cond.Type == toAddOrUpdate.Type && cond.ZoneId == toAddOrUpdate.ZoneId {
			found = true
			if cond.Status != toAddOrUpdate.Status || cond.Reason != toAddOrUpdate.Reason || cond.Msg != toAddOrUpdate.Msg {
				existing[i] = *toAddOrUpdate
				updated = true
			}
			break
		}
	}

	if !found {
		status.Conditions = append(status.Conditions, *toAddOrUpdate)
		updated = true
	}
	return updated
}

func incrementZoneVersion(application *v1.AnyApplication, ZoneId string) {
	version, err := strconv.ParseInt(application.ResourceVersion, 10, 64)
	if err != nil {
		version = 0
	}
	latestVersion := version + 1
	for _, condition := range application.Status.Conditions {
		if condition.ZoneId == ZoneId {
			conditionVersion, err := strconv.ParseInt(condition.ZoneVersion, 10, 64)
			if err != nil {
				conditionVersion = 0
			}
			if conditionVersion >= latestVersion {
				latestVersion = conditionVersion + 1
			}
		}
	}
	latestVersionStr := strconv.FormatInt(latestVersion, 10)
	for i, condition := range application.Status.Conditions {
		if condition.ZoneId == ZoneId {
			application.Status.Conditions[i].ZoneVersion = latestVersionStr
		}
	}

}
