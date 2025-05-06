package status

import (
	"context"
	"fmt"

	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdateStatus(ctx context.Context, c client.Client, name types.NamespacedName, statusUpdate func(status *v1.AnyApplicationStatus) bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var application v1.AnyApplication
		if err := c.Get(ctx, name, &application); err != nil {
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
			fmt.Printf("Updating status %v\n", updatedApplication.Status)
			return c.Status().Update(ctx, updatedApplication)
		}
		return nil // no change needed
	})
}

func AddOrUpdateCondition(ctx context.Context, c client.Client, name types.NamespacedName, toAddOrUpdate v1.ConditionStatus) error {
	return UpdateStatus(ctx, c, name, func(status *v1.AnyApplicationStatus) bool {
		return AddOrUpdate(status, &toAddOrUpdate)
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
