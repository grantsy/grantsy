package subscriptions_test

import (
	"testing"

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

func TestSubscription_IsActive_Cancelled(t *testing.T) {
	sub := &subscriptions.Subscription{Status: "cancelled"}
	assert.True(t, sub.IsActive())
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
	assert.True(t, sub.IsActive())
}
