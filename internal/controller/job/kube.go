package job

import (
	"context"

	v1 "hiro.io/anyapplication/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddOrUpdateStatusCondition(ctx context.Context, c client.Client, name types.NamespacedName, toAddOrUpdate v1.ConditionStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var application v1.AnyApplication
		if err := c.Get(ctx, name, &application); err != nil {
			if errors.IsNotFound(err) {
				return nil // Resource gone
			}
			return err
		}

		updatedApplication := application.DeepCopy()

		// Update or insert condition
		existing := updatedApplication.Status.Conditions
		updated := false
		found := false

		for i, cond := range existing {
			if cond.Type == toAddOrUpdate.Type && cond.ZoneId == toAddOrUpdate.ZoneId {
				found = true
				if cond.Status != toAddOrUpdate.Status || cond.Reason != toAddOrUpdate.Reason || cond.Msg != toAddOrUpdate.Msg {
					existing[i] = toAddOrUpdate
					updated = true
				}
				break
			}
		}

		if !found {
			updatedApplication.Status.Conditions = append(updatedApplication.Status.Conditions, toAddOrUpdate)
			updated = true
		} else {
			updatedApplication.Status.Conditions = existing
		}

		if !found || updated {
			return c.Status().Update(ctx, updatedApplication)
		}
		return nil // no change needed
	})
}
