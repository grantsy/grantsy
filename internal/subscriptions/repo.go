package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/grantsy/grantsy/internal/infra/db"
)

type Subscription struct {
	ID                      int
	UserID                  string
	CustomerID              int
	OrderID                 int
	ProductID               int
	ProductName             string
	VariantID               int
	VariantName             string
	Status                  string
	StatusFormatted         string
	CardBrand               string
	CardLastFour            string
	Cancelled               bool
	TrialEndsAt             *int64
	BillingAnchor           int
	SubscriptionItemID      int
	RenewsAt                int64
	EndsAt                  *int64
	CreatedAt               int64
	UpdatedAt               int64
	PriceID                 int
	UnitPrice               int
	RenewalIntervalUnit     string
	RenewalIntervalQuantity int
}

// IsActive returns true if the subscription grants access at the given time.
//
// When strictAccess is false (default), the lenient legacy rule applies: any of
// on_trial / active / past_due / cancelled grants access purely by status.
//
// When strictAccess is true, the strict rule applies (aligned with
// GetActiveUserPlans):
//   - active / on_trial: always active. (Cancelling during a trial immediately
//     moves the subscription to "cancelled", so on_trial never needs a guard.)
//   - cancelled: active only if this is NOT a trial cancellation (no trial, or
//     the trial already ended) AND the paid grace period is still running.
//     A trial cancellation has trial_ends_at in the future (the customer never
//     paid) and must be treated as inactive immediately.
//   - past_due: inactive — the current period's payment failed, so it is unpaid.
//   - everything else (expired, paused, unpaid, ...): inactive.
func (s *Subscription) IsActive(now int64, strictAccess bool) bool {
	if !strictAccess {
		switch s.Status {
		case "on_trial", "active", "past_due", "cancelled":
			return true
		default:
			return false
		}
	}

	switch s.Status {
	case "active", "on_trial":
		return true
	case "cancelled":
		notTrialCancel := s.TrialEndsAt == nil || *s.TrialEndsAt <= now
		inGrace := s.EndsAt == nil || now < *s.EndsAt
		return notTrialCancel && inGrace
	default:
		// past_due (strict), expired, paused, unpaid, etc.
		return false
	}
}

type Repo struct {
	db       *db.DB
	strictAccess bool
}

func NewRepo(database *db.DB, strictAccess bool) *Repo {
	return &Repo{db: database, strictAccess: strictAccess}
}

func (r *Repo) UpsertSubscription(
	ctx context.Context,
	sub *Subscription,
) error {
	table := r.db.TableName("subscriptions_lemonsqueezy")
	query := r.db.Rebind(fmt.Sprintf(`
		INSERT INTO %s (
			id, user_id, customer_id, order_id, product_id, product_name,
			variant_id, variant_name, status, status_formatted,
			card_brand, card_last_four, cancelled, trial_ends_at,
			billing_anchor, subscription_item_id, renews_at, ends_at,
			created_at, updated_at,
			price_id, unit_price, renewal_interval_unit, renewal_interval_quantity
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			customer_id = excluded.customer_id,
			order_id = excluded.order_id,
			product_id = excluded.product_id,
			product_name = excluded.product_name,
			variant_id = excluded.variant_id,
			variant_name = excluded.variant_name,
			status = excluded.status,
			status_formatted = excluded.status_formatted,
			card_brand = excluded.card_brand,
			card_last_four = excluded.card_last_four,
			cancelled = excluded.cancelled,
			trial_ends_at = excluded.trial_ends_at,
			billing_anchor = excluded.billing_anchor,
			subscription_item_id = excluded.subscription_item_id,
			renews_at = excluded.renews_at,
			ends_at = excluded.ends_at,
			updated_at = excluded.updated_at,
			price_id = excluded.price_id,
			unit_price = excluded.unit_price,
			renewal_interval_unit = excluded.renewal_interval_unit,
			renewal_interval_quantity = excluded.renewal_interval_quantity
	`, table))

	_, err := r.db.ExecContext(
		ctx,
		query,
		sub.ID,
		sub.UserID,
		sub.CustomerID,
		sub.OrderID,
		sub.ProductID,
		sub.ProductName,
		sub.VariantID,
		sub.VariantName,
		sub.Status,
		sub.StatusFormatted,
		sub.CardBrand,
		sub.CardLastFour,
		sub.Cancelled,
		sub.TrialEndsAt,
		sub.BillingAnchor,
		sub.SubscriptionItemID,
		sub.RenewsAt,
		sub.EndsAt,
		sub.CreatedAt,
		sub.UpdatedAt,
		sub.PriceID,
		sub.UnitPrice,
		sub.RenewalIntervalUnit,
		sub.RenewalIntervalQuantity,
	)
	if err != nil {
		return fmt.Errorf("billing: failed to upsert subscription: %w", err)
	}

	return nil
}

func (r *Repo) GetSubscriptionByUserID(
	ctx context.Context,
	userID string,
) (*Subscription, error) {
	table := r.db.TableName("subscriptions_lemonsqueezy")
	query := r.db.Rebind(fmt.Sprintf(`
		SELECT id, user_id, customer_id, order_id, product_id, product_name,
			variant_id, variant_name, status, status_formatted,
			card_brand, card_last_four, cancelled, trial_ends_at,
			billing_anchor, subscription_item_id, renews_at, ends_at,
			created_at, updated_at,
			price_id, unit_price, renewal_interval_unit, renewal_interval_quantity
		FROM %s
		WHERE user_id = $1
		ORDER BY
			CASE WHEN status IN ('on_trial', 'active', 'past_due', 'cancelled') THEN 0 ELSE 1 END,
			updated_at DESC
		LIMIT 1
	`, table))

	var sub Subscription
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&sub.ID, &sub.UserID, &sub.CustomerID, &sub.OrderID, &sub.ProductID, &sub.ProductName,
		&sub.VariantID, &sub.VariantName, &sub.Status, &sub.StatusFormatted,
		&sub.CardBrand, &sub.CardLastFour, &sub.Cancelled, &sub.TrialEndsAt,
		&sub.BillingAnchor, &sub.SubscriptionItemID, &sub.RenewsAt, &sub.EndsAt,
		&sub.CreatedAt, &sub.UpdatedAt,
		&sub.PriceID, &sub.UnitPrice, &sub.RenewalIntervalUnit, &sub.RenewalIntervalQuantity,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *Repo) GetActiveUserPlans(ctx context.Context) (map[string]int, error) {
	table := r.db.TableName("subscriptions_lemonsqueezy")

	// The active set must mirror Subscription.IsActive for the same strictAccess.
	var query string
	var queryArgs []any
	if !r.strictAccess {
		// Legacy (lenient): activity is decided purely by status.
		query = fmt.Sprintf(`
			SELECT user_id, product_id
			FROM %s
			WHERE product_id IS NOT NULL
			  AND status IN ('on_trial', 'active', 'past_due', 'cancelled')
			ORDER BY user_id, updated_at DESC
		`, table)
	} else {
		// Strict:
		//   active / on_trial          -> active
		//   cancelled (not a trial cancellation, still in paid grace) -> active
		//   past_due / expired / ...   -> inactive
		// Comparisons use math ordering (smaller on the left).
		query = r.db.Rebind(fmt.Sprintf(`
			SELECT user_id, product_id
			FROM %s
			WHERE product_id IS NOT NULL
			  AND (
			    status = 'active'
			    OR status = 'on_trial'
			    OR (status = 'cancelled'
			        AND (trial_ends_at IS NULL OR trial_ends_at <= $1)
			        AND (ends_at IS NULL OR $2 < ends_at))
			  )
			ORDER BY user_id, updated_at DESC
		`, table))
		now := time.Now().Unix()
		queryArgs = []any{now, now}
	}

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf(
			"subscriptions: failed to query active user plans: %w",
			err,
		)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var userID string
		var productID int
		if err := rows.Scan(&userID, &productID); err != nil {
			return nil, fmt.Errorf("subscriptions: failed to scan row: %w", err)
		}
		if _, exists := result[userID]; !exists {
			result[userID] = productID
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("subscriptions: rows error: %w", err)
	}

	return result, nil
}
