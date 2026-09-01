package subscriptions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grantsy/grantsy/internal/subscriptions"
)

func TestSubscription_IsActive_Active(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "active"}
	assert.True(t, sub.IsActive(true))
}

func TestSubscription_IsActive_OnTrial(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "on_trial"}
	assert.True(t, sub.IsActive(true))
}

func TestSubscription_IsActive_Cancelled(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "cancelled"}
	assert.True(t, sub.IsActive(true))
}

func TestSubscription_IsActive_Expired(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "expired"}
	assert.False(t, sub.IsActive(true))
}

func TestSubscription_IsActive_Paused(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "paused"}
	assert.False(t, sub.IsActive(true))
}

func TestSubscription_IsActive_PastDue(t *testing.T) {
	// past_due with no prior successful payment is inactive in strict mode
	// (trial abuser whose first charge bounced). HasSuccessfulPayment defaults
	// to false here.
	sub := &subscriptions.Subscription{Status: "past_due"}
	assert.False(t, sub.IsActive(true))
}

// A refund revokes access on its own: the provider leaves the subscription in
// whatever status it had, so every status that would otherwise grant access
// must be overridden, in both access modes.
func TestSubscription_IsActive_Refunded(t *testing.T) {
	refundedAt := int64(1717000000)

	cases := []struct {
		name                 string
		status               string
		hasSuccessfulPayment bool
	}{
		{"active", "active", true},
		{"on_trial", "on_trial", false},
		{"cancelled", "cancelled", true},
		{"past_due_paid", "past_due", true},
		{"past_due_unpaid", "past_due", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, strict := range []bool{false, true} {
				sub := &subscriptions.Subscription{
					Status:               tc.status,
					HasSuccessfulPayment: tc.hasSuccessfulPayment,
					RefundedAt:           &refundedAt,
				}
				assert.False(t, sub.IsActive(strict), "strictAccess=%v", strict)
			}
		})
	}
}

// Guards the past_due branch that the refund check sits in front of: without a
// refund, a paid-up past_due subscription keeps access in both modes.
func TestSubscription_IsActive_PastDueWithSuccessfulPayment(t *testing.T) {
	sub := &subscriptions.Subscription{
		Status:               "past_due",
		HasSuccessfulPayment: true,
	}
	assert.True(t, sub.IsActive(true))
	assert.True(t, sub.IsActive(false))
}
