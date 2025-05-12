package status

import (
	"context"
	"sync/atomic"

	"github.com/go-logr/logr"
	v1 "hiro.io/anyapplication/api/v1"
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
}

func NewStatusUpdater(
	ctx context.Context,
	log logr.Logger,
	client client.Client,
	name types.NamespacedName,
) StatusUpdater {
	return StatusUpdater{
		ctx:    ctx,
		log:    log,
		client: client,
		name:   name,
	}
}

func (su *StatusUpdater) UpdateStatus(
	stopRetrying *atomic.Bool,
	statusUpdate func(status *v1.AnyApplicationStatus) bool,
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
		if statusUpdate != nil {
			updated = statusUpdate(&updatedApplication.Status)
		}
		// Update or insert condition
		if updated {
			if stopRetrying != nil && stopRetrying.Load() {
				return nil // stop retrying
			}

			err := su.client.Status().Update(su.ctx, updatedApplication)
			su.log.Info("Updating status", "status", updatedApplication.Status, "error", err)
			return err
		}
		return nil // no change needed
	})
}

func (su *StatusUpdater) UpdateCondition(
	stopRetrying *atomic.Bool,
	conditionToUpdate v1.ConditionStatus,
) error {
	return su.UpdateStatus(stopRetrying, func(status *v1.AnyApplicationStatus) bool {
		return AddOrUpdate(status, &conditionToUpdate)
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
