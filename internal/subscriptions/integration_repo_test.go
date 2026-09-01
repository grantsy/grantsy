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
					repo := drv.newDB(t, true)
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
					repo := drv.newDB(t, true)
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
					repo := drv.newDB(t, true)
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
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Equal(t, "user-1", got.UserID)
				})

				t.Run("not_found", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					got, err := repo.GetSubscriptionByUserID(ctx, "nonexistent")
					require.NoError(t, err)
					assert.Nil(t, got)
				})

				t.Run("nil_optional_fields", func(t *testing.T) {
					repo := drv.newDB(t, true)
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
					repo := drv.newDB(t, true)
					ctx := context.Background()

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("active_subscription", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				t.Run("on_trial_subscription", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "on_trial")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				t.Run("cancelled_with_future_ends_at", func(t *testing.T) {
					repo := drv.newDB(t, true)
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
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "expired")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("paused_subscription_excluded", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "paused")
					err := repo.UpsertSubscription(ctx, sub)
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("multiple_users_mixed_statuses", func(t *testing.T) {
					repo := drv.newDB(t, true)
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

			t.Run("Refunds", func(t *testing.T) {
				// The regular upsert must never clear refunded_at, and must read
				// it back: LemonSqueezy keeps sending subscription_updated with
				// status "active" after a refund, and an in-memory sub with a
				// nil RefundedAt would report IsActive and hand the plan back.
				t.Run("upsert_preserves_refunded_at", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))

					refundedAt := time.Now().Unix()
					_, err := repo.MarkRefundedBySubscriptionID(ctx, 1, refundedAt)
					require.NoError(t, err)

					// A routine update arriving after the refund.
					fresh := testSub(1, "user-1", "active")
					require.Nil(t, fresh.RefundedAt)
					require.NoError(t, repo.UpsertSubscription(ctx, fresh))

					require.NotNil(t, fresh.RefundedAt, "upsert must return the persisted refunded_at")
					assert.Equal(t, refundedAt, *fresh.RefundedAt)
					assert.False(t, fresh.IsActive(true))

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					require.NotNil(t, got.RefundedAt)
					assert.Equal(t, refundedAt, *got.RefundedAt)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Empty(t, plans)
				})

				t.Run("mark_by_subscription_id", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))

					refundedAt := time.Now().Unix()
					got, err := repo.MarkRefundedBySubscriptionID(ctx, 1, refundedAt)
					require.NoError(t, err)
					require.NotNil(t, got)
					assert.Equal(t, "user-1", got.UserID)
					assert.Equal(t, 12345, got.ProductID)
					require.NotNil(t, got.RefundedAt)
					assert.Equal(t, refundedAt, *got.RefundedAt)
				})

				t.Run("mark_by_order_id", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					sub := testSub(1, "user-1", "active")
					require.NoError(t, repo.UpsertSubscription(ctx, sub))

					refundedAt := time.Now().Unix()
					got, err := repo.MarkRefundedByOrderID(ctx, sub.OrderID, refundedAt)
					require.NoError(t, err)
					require.NotNil(t, got)
					assert.Equal(t, 1, got.ID)
					assert.Equal(t, "user-1", got.UserID)
					require.NotNil(t, got.RefundedAt)
					assert.Equal(t, refundedAt, *got.RefundedAt)
				})

				// A refund for something this service does not track is not an
				// error — the caller distinguishes it by the nil subscription.
				t.Run("unknown_subscription_id", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					got, err := repo.MarkRefundedBySubscriptionID(ctx, 999, time.Now().Unix())
					require.NoError(t, err)
					assert.Nil(t, got)
				})

				t.Run("unknown_order_id", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					got, err := repo.MarkRefundedByOrderID(ctx, 999, time.Now().Unix())
					require.NoError(t, err)
					assert.Nil(t, got)
				})

				// Providers redeliver webhooks: a repeat must keep the original
				// timestamp and still return the subscription.
				t.Run("idempotent", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))

					first := time.Now().Unix()
					_, err := repo.MarkRefundedBySubscriptionID(ctx, 1, first)
					require.NoError(t, err)

					got, err := repo.MarkRefundedBySubscriptionID(ctx, 1, first+3600)
					require.NoError(t, err)
					require.NotNil(t, got)
					require.NotNil(t, got.RefundedAt)
					assert.Equal(t, first, *got.RefundedAt)
				})

				t.Run("excluded_from_active_plans_strict", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-refunded", "active")))
					require.NoError(t, repo.UpsertSubscription(ctx, testSub(2, "user-ok", "active")))

					_, err := repo.MarkRefundedBySubscriptionID(ctx, 1, time.Now().Unix())
					require.NoError(t, err)

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-ok": 12345}, plans)
				})

				// The lenient predicate is a separate SQL branch, and past_due
				// only grants access there, so it needs its own case.
				t.Run("excluded_from_active_plans_lenient", func(t *testing.T) {
					repo := drv.newDB(t, false)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-refunded", "past_due")))
					require.NoError(t, repo.UpsertSubscription(ctx, testSub(2, "user-ok", "past_due")))

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Len(t, plans, 2, "past_due grants access in lenient mode")

					_, err = repo.MarkRefundedBySubscriptionID(ctx, 1, time.Now().Unix())
					require.NoError(t, err)

					plans, err = repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-ok": 12345}, plans)
				})

				// Re-subscribing after a refund produces a new provider
				// subscription ID for the same user. The unique
				// one-active-subscription-per-user index must let it in, which
				// it only does because refunded rows are outside the index.
				t.Run("repurchase_after_refund", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))
					_, err := repo.MarkRefundedBySubscriptionID(ctx, 1, time.Now().Unix())
					require.NoError(t, err)

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(2, "user-1", "active")))

					plans, err := repo.GetActiveUserPlans(ctx)
					require.NoError(t, err)
					assert.Equal(t, map[string]int{"user-1": 12345}, plans)
				})

				// A refunded row must not outrank a live one for the same user.
				t.Run("ranks_below_live_subscription", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))
					_, err := repo.MarkRefundedBySubscriptionID(ctx, 1, time.Now().Unix())
					require.NoError(t, err)
					require.NoError(t, repo.UpsertSubscription(ctx, testSub(2, "user-1", "active")))

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Equal(t, 2, got.ID)
					assert.Nil(t, got.RefundedAt)
				})

				t.Run("nil_when_never_refunded", func(t *testing.T) {
					repo := drv.newDB(t, true)
					ctx := context.Background()

					require.NoError(t, repo.UpsertSubscription(ctx, testSub(1, "user-1", "active")))

					got, err := repo.GetSubscriptionByUserID(ctx, "user-1")
					require.NoError(t, err)
					assert.Nil(t, got.RefundedAt)
					assert.True(t, got.IsActive(true))
				})
			})
		})
	}
}
