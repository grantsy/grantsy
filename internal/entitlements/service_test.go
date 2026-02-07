package entitlements_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/entitlements/mocks"
	"github.com/grantsy/grantsy/internal/infra/config"
)

func testEntitlementsConfig() *config.EntitlementsConfig {
	return &config.EntitlementsConfig{
		DefaultPlan: "free",
		Plans: []config.PlanConfig{
			{ID: "free", Name: "Free", Features: []string{"dashboard"}},
			{ID: "pro", Name: "Pro", Features: []string{"dashboard", "api", "sso"}},
		},
		Features: []config.FeatureConfig{
			{ID: "dashboard", Name: "Dashboard", Description: "Basic dashboard access"},
			{ID: "api", Name: "API", Description: "API access"},
			{ID: "sso", Name: "SSO", Description: "Single sign-on"},
		},
	}
}

func testProducts() []config.ProductMapping {
	return []config.ProductMapping{
		{ProductID: 100, PlanID: "pro"},
	}
}

func newTestService(t *testing.T, loader entitlements.SubscriptionLoader, notifier entitlements.PlanUpdateNotifier) *entitlements.Service {
	t.Helper()
	svc, err := entitlements.NewService(testEntitlementsConfig(), testProducts(), loader, notifier)
	require.NoError(t, err)
	return svc
}

func newEmptyLoader(t *testing.T) *mocks.MockSubscriptionLoader {
	t.Helper()
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{}, nil)
	return loader
}

// --- NewService ---

func TestNewService_Success(t *testing.T) {
	loader := newEmptyLoader(t)
	svc := newTestService(t, loader, nil)

	plans := svc.GetPlans()
	assert.Len(t, plans, 2)
	assert.Equal(t, "free", plans[0].ID)
	assert.Equal(t, "pro", plans[1].ID)
}

func TestNewService_SubscriptionLoaderError(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(nil, errors.New("db error"))

	_, err := entitlements.NewService(testEntitlementsConfig(), testProducts(), loader, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load subscriptions")
}

func TestNewService_LoadsExistingSubscriptions(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"user1": 100}, nil)

	svc := newTestService(t, loader, nil)
	assert.Equal(t, "pro", svc.GetUserPlan("user1"))
}

func TestNewService_UnknownProductID(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"user1": 999}, nil)

	svc := newTestService(t, loader, nil)
	assert.Equal(t, "free", svc.GetUserPlan("user1"))
}

// --- CheckFeature ---

func TestCheckFeature_AllowedSubscribedUser(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"user1": 100}, nil)

	svc := newTestService(t, loader, nil)
	result := svc.CheckFeature("user1", "api")

	assert.True(t, result.Allowed)
	assert.Equal(t, "feature_in_plan", result.Reason)
	assert.Equal(t, "api", result.FeatureID)
	assert.Equal(t, "user1", result.UserID)
	assert.Equal(t, "pro", result.PlanID)
}

func TestCheckFeature_AllowedDefaultPlan(t *testing.T) {
	// Users without subscription get the default plan. Features in the default plan
	// are granted via direct feature list check (no Casbin grouping needed).
	svc := newTestService(t, newEmptyLoader(t), nil)
	result := svc.CheckFeature("user1", "dashboard")

	assert.True(t, result.Allowed)
	assert.Equal(t, "default_plan", result.Reason)
	assert.Equal(t, "free", result.PlanID)
}

func TestCheckFeature_DeniedInsufficientPlan(t *testing.T) {
	// Pro user checking a feature not in their plan won't happen with current config
	// (pro has all features), so test a free-plan user checking a pro-only feature.
	svc := newTestService(t, newEmptyLoader(t), nil)
	result := svc.CheckFeature("user1", "api")

	assert.False(t, result.Allowed)
	assert.Equal(t, "insufficient_plan", result.Reason)
	assert.Equal(t, "free", result.PlanID)
}

func TestCheckFeature_DeniedNoDefaultPlan(t *testing.T) {
	cfg := testEntitlementsConfig()
	cfg.DefaultPlan = ""
	loader := newEmptyLoader(t)

	svc, err := entitlements.NewService(cfg, testProducts(), loader, nil)
	require.NoError(t, err)

	result := svc.CheckFeature("user1", "dashboard")
	assert.False(t, result.Allowed)
	assert.Equal(t, "no_subscription", result.Reason)
	assert.Equal(t, "", result.PlanID)
}

// --- GetUserPlan ---

func TestGetUserPlan_WithSubscription(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"user1": 100}, nil)

	svc := newTestService(t, loader, nil)
	assert.Equal(t, "pro", svc.GetUserPlan("user1"))
}

func TestGetUserPlan_DefaultPlan(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	assert.Equal(t, "free", svc.GetUserPlan("user1"))
}

func TestGetUserPlan_NoDefaultPlan(t *testing.T) {
	cfg := testEntitlementsConfig()
	cfg.DefaultPlan = ""
	loader := newEmptyLoader(t)

	svc, err := entitlements.NewService(cfg, testProducts(), loader, nil)
	require.NoError(t, err)

	assert.Equal(t, "", svc.GetUserPlan("user1"))
}

// --- GetUserFeatures ---

func TestGetUserFeatures_WithPlan(t *testing.T) {
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"user1": 100}, nil)

	svc := newTestService(t, loader, nil)
	features := svc.GetUserFeatures("user1")
	assert.Equal(t, []string{"dashboard", "api", "sso"}, features)
}

func TestGetUserFeatures_DefaultPlan(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	features := svc.GetUserFeatures("user1")
	assert.Equal(t, []string{"dashboard"}, features)
}

func TestGetUserFeatures_UnknownPlan(t *testing.T) {
	cfg := testEntitlementsConfig()
	cfg.DefaultPlan = ""
	loader := newEmptyLoader(t)

	svc, err := entitlements.NewService(cfg, testProducts(), loader, nil)
	require.NoError(t, err)

	features := svc.GetUserFeatures("user1")
	assert.Equal(t, []string{}, features)
}

// --- OnSubscriptionChange ---

func TestOnSubscriptionChange_Activate(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 100, true, nil)
	require.NoError(t, err)

	assert.Equal(t, "pro", svc.GetUserPlan("user1"))
}

func TestOnSubscriptionChange_Deactivate(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 100, true, nil)
	require.NoError(t, err)
	assert.Equal(t, "pro", svc.GetUserPlan("user1"))

	err = svc.OnSubscriptionChange(context.Background(), "user1", 0, false, nil)
	require.NoError(t, err)
	assert.Equal(t, "free", svc.GetUserPlan("user1"))
}

func TestOnSubscriptionChange_NotifierCalled(t *testing.T) {
	notifier := mocks.NewMockPlanUpdateNotifier(t)
	notifier.EXPECT().
		NotifyPlanUpdated(mock.Anything, "user1", "pro", "free", mock.Anything).
		Return(nil)

	svc := newTestService(t, newEmptyLoader(t), notifier)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 100, true, nil)
	require.NoError(t, err)
}

func TestOnSubscriptionChange_NotifierNil(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 100, true, nil)
	require.NoError(t, err)
	assert.Equal(t, "pro", svc.GetUserPlan("user1"))
}

func TestOnSubscriptionChange_NotifierError(t *testing.T) {
	notifier := mocks.NewMockPlanUpdateNotifier(t)
	notifier.EXPECT().
		NotifyPlanUpdated(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("webhook error"))

	svc := newTestService(t, newEmptyLoader(t), notifier)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 100, true, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook error")
}

func TestOnSubscriptionChange_UnknownProduct(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)

	err := svc.OnSubscriptionChange(context.Background(), "user1", 999, true, nil)
	require.NoError(t, err)

	assert.Equal(t, "free", svc.GetUserPlan("user1"))
}

// --- GetPlans, GetFeatures, GetPlan ---

func TestGetPlans(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	plans := svc.GetPlans()

	require.Len(t, plans, 2)
	assert.Equal(t, "free", plans[0].ID)
	assert.Equal(t, "pro", plans[1].ID)
}

func TestGetFeatures(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	features := svc.GetFeatures()

	require.Len(t, features, 3)
	assert.Equal(t, "dashboard", features[0].ID)
	assert.Equal(t, "api", features[1].ID)
	assert.Equal(t, "sso", features[2].ID)
}

func TestGetPlan_Exists(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	plan := svc.GetPlan("pro")

	require.NotNil(t, plan)
	assert.Equal(t, "pro", plan.ID)
	assert.Equal(t, "Pro", plan.Name)
}

func TestGetPlan_NotExists(t *testing.T) {
	svc := newTestService(t, newEmptyLoader(t), nil)
	plan := svc.GetPlan("enterprise")

	assert.Nil(t, plan)
}
