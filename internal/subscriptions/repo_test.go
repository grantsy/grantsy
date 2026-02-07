package subscriptions_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/grantsy/grantsy/internal/subscriptions"
)

func TestSubscription_IsActive_Active(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "active"}
	assert.True(t, sub.IsActive())
}

func TestSubscription_IsActive_OnTrial(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "on_trial"}
	assert.True(t, sub.IsActive())
}

func TestSubscription_IsActive_CancelledFutureEnd(t *testing.T) {
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	sub := &subscriptions.Subscription{Status: "cancelled", EndsAt: &future}
	assert.True(t, sub.IsActive())
}

func TestSubscription_IsActive_CancelledPastEnd(t *testing.T) {
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	sub := &subscriptions.Subscription{Status: "cancelled", EndsAt: &past}
	assert.False(t, sub.IsActive())
}

func TestSubscription_IsActive_CancelledNilEnd(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "cancelled", EndsAt: nil}
	assert.False(t, sub.IsActive())
}

func TestSubscription_IsActive_Expired(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "expired"}
	assert.False(t, sub.IsActive())
}

func TestSubscription_IsActive_Paused(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "paused"}
	assert.False(t, sub.IsActive())
}

func TestSubscription_IsActive_PastDue(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "past_due"}
	assert.False(t, sub.IsActive())
}
