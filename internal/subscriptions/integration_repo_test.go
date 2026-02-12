package subscriptions_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoIntegration(t *testing.T) {
	for _, drv := range drivers {
		t.Run(drv.name, func(t *testing.T) {
			t.Run("UpsertSubscription", func(t *testing.T) {
				t.Run("insert_new", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Equal(t, 1, got.ID)
					assert.Equal(t, "user-1", got.UserID)
					assert.Equal(t, "active", got.Status)
					assert.Equal(t, 12345, got.ProductID)
				})

				t.Run("update_existing", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					sub.Status = "cancelled"
					sub.Cancelled = true
					endsAt := time.Now().Add(30 * 24 * time.Hour).Unix()
					sub.EndsAt = &endsAt
					sub.UpdatedAt = time.Now().Unix()
					err = repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Equal(t, "cancelled", got.Status)
					assert.True(t, got.Cancelled)
					require.NotNil(t, got.EndsAt)
					assert.Equal(t, endsAt, *got.EndsAt)
				})

				t.Run("all_fields_roundtrip", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					trialEnd := time.Now().Add(14 * 24 * time.Hour).Unix()
					endsAt := time.Now().Add(30 * 24 * time.Hour).Unix()
					sub := testSub(42, "user-rt", "on_trial")
					sub.TrialEndsAt = &trialEnd
					sub.EndsAt = &endsAt

					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-rt")
					require.NoError(t, err)

					assert.Equal(t, sub.ID, got.ID)
					assert.Equal(t, sub.UserID, got.UserID)
					assert.Equal(t, sub.CustomerID, got.CustomerID)
					assert.Equal(t, sub.OrderID, got.OrderID)
					assert.Equal(t, sub.ProductID, got.ProductID)
					assert.Equal(t, sub.ProductName, got.ProductName)
					assert.Equal(t, sub.VariantID, got.VariantID)
					assert.Equal(t, sub.VariantName, got.VariantName)
					assert.Equal(t, sub.Status, got.Status)
					assert.Equal(t, sub.StatusFormatted, got.StatusFormatted)
					assert.Equal(t, sub.CardBrand, got.CardBrand)
					assert.Equal(t, sub.CardLastFour, got.CardLastFour)
					assert.Equal(t, sub.Cancelled, got.Cancelled)
					require.NotNil(t, got.TrialEndsAt)
					assert.Equal(t, *sub.TrialEndsAt, *got.TrialEndsAt)
					assert.Equal(t, sub.BillingAnchor, got.BillingAnchor)
					assert.Equal(t, sub.SubscriptionItemID, got.SubscriptionItemID)
					assert.Equal(t, sub.RenewsAt, got.RenewsAt)
					require.NotNil(t, got.EndsAt)
					assert.Equal(t, *sub.EndsAt, *got.EndsAt)
					assert.Equal(t, sub.CreatedAt, got.CreatedAt)
					assert.Equal(t, sub.UpdatedAt, got.UpdatedAt)
				})
			})

			t.Run("GetSubscriptionByUserID", func(t *testing.T) {
				t.Run("found", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Equal(t, "user-1", got.UserID)
				})

				t.Run("not_found", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					got, err := repo.GetSubscriptionByUserID(ctx, "nonexistent")
					require.NoError(t, err)
					assert.Nil(t, got)
				})

				t.Run("nil_optional_fields", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					sub.TrialEndsAt = nil
					sub.EndsAt = nil
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Nil(t, got.TrialEndsAt)
					assert.Nil(t, got.EndsAt)
				})
			})

			t.Run("GetActiveUserPlans", func(t *testing.T) {
				t.Run("empty_table", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("active_subscription", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				t.Run("on_trial_subscription", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "on_trial")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				t.Run("cancelled_with_future_ends_at", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					future := time.Now().Add(30 * 24 * time.Hour).Unix()
					sub := testSub(1, "user-1", "cancelled")
					sub.EndsAt = &future
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				t.Run("expired_subscription_excluded", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "expired")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("paused_subscription_excluded", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub := testSub(1, "user-1", "paused")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("multiple_users_mixed_statuses", func(t *testing.T) {
					repo := drv.newDB(t)
					ctx := context.Background()

					sub1 := testSub(1, "user-active", "active")
					require.NoError(t, repo.UpsertSubscription(ctx, sub1))

					sub2 := testSub(2, "user-trial", "on_trial")
					require.NoError(t, repo.UpsertSubscription(ctx, sub2))

					sub3 := testSub(3, "user-expired", "expired")
					require.NoError(t, repo.UpsertSubscription(ctx, sub3))

					future := time.Now().Add(30 * 24 * time.Hour).Unix()
					sub4 := testSub(4, "user-cancelled-future", "cancelled")
					sub4.EndsAt = &future
					require.NoError(t, repo.UpsertSubscription(ctx, sub4))

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Len(t, plans, 3)
					assert.Equal(t, 12345, plans["user-active"])
					assert.Equal(t, 12345, plans["user-trial"])
					assert.Equal(t, 12345, plans["user-cancelled-future"])
					_, exists := plans["user-expired"]
					assert.False(t, exists)
				})
			})
		})
	}
}
