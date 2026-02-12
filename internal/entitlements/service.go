package entitlements

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/grantsy/grantsy/internal/infra/config"
	"github.com/grantsy/grantsy/internal/infra/metrics"
)

// SubscriptionLoader provides active subscription mappings for entitlements initialization.
type SubscriptionLoader interface {
	// GetActiveUserPlans returns a map of userID -> productID for all active subscriptions.
	// Includes subscriptions with status 'active', 'on_trial', or 'cancelled' (if ends_at is in the future).
	GetActiveUserPlans(ctx context.Context) (map[string]int, error)
}

// PricingProvider supplies variant/pricing data for plans.
type PricingProvider interface {
	GetPlanVariants(planID string) []Variant
}

// PlanUpdateNotifier is called when a user's plan changes
type PlanUpdateNotifier interface {
	NotifyPlanUpdated(
		ctx context.Context,
		userID, activePlan, prevPlan string,
		subscription any,
	) error
}

//go:embed casbin_model.conf
var casbinModel string

type Service struct {
	enforcer            *casbin.Enforcer
	ent                 *config.EntitlementsConfig
	subLoader           SubscriptionLoader
	notifier            PlanUpdateNotifier
	mu                  sync.RWMutex
	plansByID           map[string]*config.PlanConfig
	featuresByID        map[string]*config.FeatureConfig
	productToPlan       map[int]string
	defaultPlanFeatures map[string]struct{}
}

type CheckReason string

const (
	ReasonNoSubscription   CheckReason = "no_subscription"
	ReasonDefaultPlan      CheckReason = "default_plan"
	ReasonFeatureInPlan    CheckReason = "feature_in_plan"
	ReasonInsufficientPlan CheckReason = "insufficient_plan"
)

type CheckResult struct {
	Allowed   bool
	FeatureID string
	UserID    string
	PlanID    string
	Reason    CheckReason
}

func NewService(
	ent *config.EntitlementsConfig,
	products []config.ProductMapping,
	subLoader SubscriptionLoader,
	notifier PlanUpdateNotifier,
) (*Service, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("entitlements: failed to load casbin model: %w", err)
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("entitlements: failed to create enforcer: %w", err)
	}

	s := &Service{
		enforcer:            e,
		ent:                 ent,
		subLoader:           subLoader,
		notifier:            notifier,
		plansByID:           make(map[string]*config.PlanConfig, len(ent.Plans)),
		featuresByID:        make(map[string]*config.FeatureConfig, len(ent.Features)),
		productToPlan:       make(map[int]string, len(products)),
		defaultPlanFeatures: make(map[string]struct{}),
	}

	s.buildLookups(products)

	if err := s.loadPolicies(); err != nil {
		return nil, fmt.Errorf("entitlements: failed to load policies: %w", err)
	}

	if err := s.loadSubscriptions(context.Background()); err != nil {
		return nil, fmt.Errorf(
			"entitlements: failed to load subscriptions: %w",
			err,
		)
	}

	s.updateSubscriptionMetrics()

	return s, nil
}

func (s *Service) loadPolicies() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, plan := range s.ent.Plans {
		for _, featureID := range plan.Features {
			if _, err := s.enforcer.AddPolicy(plan.ID, featureID, "access"); err != nil {
				return fmt.Errorf(
					"failed to add policy for plan %s feature %s: %w",
					plan.ID,
					featureID,
					err,
				)
			}
		}
	}

	return nil
}

func (s *Service) loadSubscriptions(ctx context.Context) error {
	userPlans, err := s.subLoader.GetActiveUserPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for userID, productID := range userPlans {
		planID := s.resolvePlanFromProduct(productID)
		if planID != "" {
			if _, err := s.enforcer.AddGroupingPolicy(userID, planID); err != nil {
				return fmt.Errorf(
					"failed to add grouping for user %s: %w",
					userID,
					err,
				)
			}
		}
	}

	return nil
}

func (s *Service) CheckFeature(userID, featureID string) *CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	planID := s.getUserPlan(userID)
	allowed, _ := s.enforcer.Enforce(userID, featureID, "access")

	// If Casbin denies but user is on the default plan, check plan features directly.
	// Default plan users have no Casbin grouping, so Enforce always returns false for them.
	if !allowed && planID == s.ent.DefaultPlan && s.ent.DefaultPlan != "" {
		_, allowed = s.defaultPlanFeatures[featureID]
	}

	var reason CheckReason
	if planID == "" {
		reason = ReasonNoSubscription
	} else if allowed {
		if s.ent.DefaultPlan != "" && planID == s.ent.DefaultPlan {
			reason = ReasonDefaultPlan
		} else {
			reason = ReasonFeatureInPlan
		}
	} else {
		reason = ReasonInsufficientPlan
	}

	return &CheckResult{
		Allowed:   allowed,
		FeatureID: featureID,
		UserID:    userID,
		PlanID:    planID,
		Reason:    reason,
	}
}

func (s *Service) getUserPlan(userID string) string {
	roles, _ := s.enforcer.GetRolesForUser(userID)
	if len(roles) > 0 {
		return roles[0]
	}
	return s.ent.DefaultPlan
}

func (s *Service) GetUserPlan(userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUserPlan(userID)
}

func (s *Service) GetUserFeatures(userID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	planID := s.getUserPlan(userID)
	plan := s.GetPlan(planID)
	if plan == nil {
		return []string{}
	}
	return plan.Features
}

// OnSubscriptionChange handles subscription state changes.
// Implements subscriptions.SubscriptionObserver interface.
func (s *Service) OnSubscriptionChange(
	ctx context.Context,
	userID string,
	productID int,
	active bool,
	subscription any,
) error {
	// Get previous plan before any changes
	prevPlan := s.GetUserPlan(userID)

	if active && productID != 0 {
		if err := s.activateUser(userID, productID); err != nil {
			return err
		}
	} else if !active {
		// Expired, paused, cancelled, etc. - deactivate
		if err := s.deactivateUser(userID); err != nil {
			return err
		}
	}

	// Get current plan after update
	activePlan := s.GetUserPlan(userID)

	// Notify webhooks
	if s.notifier != nil {
		return s.notifier.NotifyPlanUpdated(
			ctx,
			userID,
			activePlan,
			prevPlan,
			subscription,
		)
	}
	return nil
}

// activateUser assigns a plan to a user based on productID.
func (s *Service) activateUser(userID string, productID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing plan first
	if _, err := s.enforcer.DeleteRolesForUser(userID); err != nil {
		return fmt.Errorf("failed to delete roles for user %s: %w", userID, err)
	}

	planID := s.resolvePlanFromProduct(productID)
	if planID == "" {
		return nil
	}

	if _, err := s.enforcer.AddGroupingPolicy(userID, planID); err != nil {
		return fmt.Errorf("failed to add grouping for user %s: %w", userID, err)
	}
	s.updateSubscriptionMetrics()
	return nil
}

// deactivateUser removes all plan assignments for a user.
func (s *Service) deactivateUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.enforcer.DeleteRolesForUser(userID); err != nil {
		return fmt.Errorf("failed to delete roles for user %s: %w", userID, err)
	}
	s.updateSubscriptionMetrics()
	return nil
}

func (s *Service) GetPlans() []config.PlanConfig {
	return s.ent.Plans
}

func (s *Service) GetFeatures() []config.FeatureConfig {
	return s.ent.Features
}

func (s *Service) GetPlan(planID string) *config.PlanConfig {
	return s.plansByID[planID]
}

func (s *Service) GetFeature(featureID string) *config.FeatureConfig {
	return s.featuresByID[featureID]
}

func (s *Service) resolvePlanFromProduct(productID int) string {
	return s.productToPlan[productID]
}

func (s *Service) buildLookups(products []config.ProductMapping) {
	for i := range s.ent.Plans {
		s.plansByID[s.ent.Plans[i].ID] = &s.ent.Plans[i]
	}
	for i := range s.ent.Features {
		s.featuresByID[s.ent.Features[i].ID] = &s.ent.Features[i]
	}
	for _, mapping := range products {
		s.productToPlan[mapping.ProductID] = mapping.PlanID
	}
	if plan := s.plansByID[s.ent.DefaultPlan]; plan != nil {
		for _, f := range plan.Features {
			s.defaultPlanFeatures[f] = struct{}{}
		}
	}
}

func (s *Service) getSubscriptionCountsByPlan() map[string]int {
	counts := make(map[string]int)
	for _, plan := range s.ent.Plans {
		users, _ := s.enforcer.GetUsersForRole(plan.ID)
		counts[plan.ID] = len(users)
	}
	return counts
}

func (s *Service) updateSubscriptionMetrics() {
	counts := s.getSubscriptionCountsByPlan()
	for planID, count := range counts {
		metrics.SetActiveSubscriptions(planID, float64(count))
	}
}
