package subscriptions

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/iamolegga/lemonsqueezy-go"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/infra/config"
)

// LemonSqueezyProvider consolidates all LemonSqueezy SDK usage.
// It fetches and caches variant/pricing data and verifies webhooks.
type LemonSqueezyProvider struct {
	client        *lemonsqueezy.Client
	productToPlan map[int]string
	mu            sync.RWMutex
	cache         map[string][]entitlements.Variant
}

func NewLemonSqueezyProvider(
	apiKey string,
	signingSecret string,
	products []config.ProductMapping,
) *LemonSqueezyProvider {
	productToPlan := make(map[int]string, len(products))
	for _, p := range products {
		productToPlan[p.ProductID] = p.PlanID
	}

	return &LemonSqueezyProvider{
		client: lemonsqueezy.New(
			lemonsqueezy.WithAPIKey(apiKey),
			lemonsqueezy.WithSigningSecret(signingSecret),
		),
		productToPlan: productToPlan,
		cache:         make(map[string][]entitlements.Variant),
	}
}

// Start loads pricing data and optionally refreshes it periodically.
// If syncPeriod is 0, it loads once and returns.
func (p *LemonSqueezyProvider) Start(
	ctx context.Context,
	syncPeriod time.Duration,
) {
	p.load(ctx)

	if syncPeriod <= 0 {
		return
	}

	ticker := time.NewTicker(syncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.load(ctx)
		}
	}
}

func (p *LemonSqueezyProvider) load(ctx context.Context) {
	resp, _, err := p.client.Products.ListWithVariants(ctx)
	if err != nil {
		slog.Error(
			"failed to fetch products with variants from LemonSqueezy",
			"error",
			err,
		)
		return
	}

	cache := make(map[string][]entitlements.Variant)

	for _, variant := range resp.Included {
		if variant.Attributes.Status != "published" {
			continue
		}

		planID, ok := p.productToPlan[variant.Attributes.ProductID]
		if !ok {
			continue
		}

		id, _ := strconv.Atoi(variant.ID)

		var interval string
		if variant.Attributes.Interval != nil {
			interval = *variant.Attributes.Interval
		}
		var intervalCount int
		if variant.Attributes.IntervalCount != nil {
			intervalCount = *variant.Attributes.IntervalCount
		}

		cache[planID] = append(cache[planID], entitlements.Variant{
			ID:                 id,
			Name:               variant.Attributes.Name,
			Price:              variant.Attributes.Price.(int),
			Interval:           interval,
			IntervalCount:      intervalCount,
			HasFreeTrial:       variant.Attributes.HasFreeTrial,
			TrialInterval:      variant.Attributes.TrialInterval,
			TrialIntervalCount: variant.Attributes.TrialIntervalCount,
			Sort:               variant.Attributes.Sort,
		})
	}

	for _, variants := range cache {
		sort.Slice(variants, func(i, j int) bool {
			return variants[i].Sort < variants[j].Sort
		})
	}

	p.mu.Lock()
	p.cache = cache
	p.mu.Unlock()

	slog.Info("loaded pricing variants from LemonSqueezy", "plans", len(cache))
}

// GetPlanVariants returns cached variant data for the given plan.
func (p *LemonSqueezyProvider) GetPlanVariants(
	planID string,
) []entitlements.Variant {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cache[planID]
}

// PriceInfo holds price data fetched from LemonSqueezy.
type PriceInfo struct {
	UnitPrice               int
	RenewalIntervalUnit     string
	RenewalIntervalQuantity int
}

// GetPrice fetches price data from LemonSqueezy by price ID.
func (p *LemonSqueezyProvider) GetPrice(
	ctx context.Context,
	priceID int,
) (*PriceInfo, error) {
	resp, _, err := p.client.Prices.Get(ctx, priceID)
	if err != nil {
		return nil, fmt.Errorf("lemonsqueezy: failed to get price %d: %w", priceID, err)
	}

	return &PriceInfo{
		UnitPrice:               resp.Data.Attributes.UnitPrice,
		RenewalIntervalUnit:     resp.Data.Attributes.RenewalIntervalUnit,
		RenewalIntervalQuantity: resp.Data.Attributes.RenewalIntervalQuantity,
	}, nil
}

// VerifyWebhook validates a LemonSqueezy webhook signature.
func (p *LemonSqueezyProvider) VerifyWebhook(
	ctx context.Context,
	signature string,
	body []byte,
) bool {
	return p.client.Webhooks.Verify(ctx, signature, body)
}
