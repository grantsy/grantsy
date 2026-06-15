package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	// HasSuccessfulPayment is a sticky flag set once the subscription has been
	// observed in status "active" (i.e. at least one charge succeeded). Used to
	// keep past_due access for paying customers in strict mode. Internal only —
	// not exposed via the API.
	HasSuccessfulPayment bool
}

// IsActive returns true if the subscription grants access.
//
// on_trial / active / cancelled always grant access (cancelled keeps access
// through its grace period; LemonSqueezy moves it to "expired" once that ends).
//
// strictAccess only affects past_due. In lenient mode (false) past_due keeps
// access during dunning. In strict mode (true) past_due keeps access only if
// the subscription has had a successful payment (HasSuccessfulPayment) — this
// preserves the dunning grace for paying customers while cutting off trial
// abusers whose very first charge bounced (they never reached "active").
//
// Everything else (expired, paused, unpaid, ...) is inactive.
//
// NOTE: the past_due rule is mirrored in Repo.GetActiveUserPlans (SQL) — keep
// the two in sync.
func (s *Subscription) IsActive(strictAccess bool) bool {
	switch s.Status {
	case "on_trial", "active", "cancelled":
		return true
	case "past_due":
		return !strictAccess || s.HasSuccessfulPayment
	default:
		return false
	}
}

type Repo struct {
	db           *db.DB
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
		INSERT INTO %[1]s (
			id, user_id, customer_id, order_id, product_id, product_name,
			variant_id, variant_name, status, status_formatted,
			card_brand, card_last_four, cancelled, trial_ends_at,
			billing_anchor, subscription_item_id, renews_at, ends_at,
			created_at, updated_at,
			price_id, unit_price, renewal_interval_unit, renewal_interval_quantity,
			has_successful_payment
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
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
			renewal_interval_quantity = excluded.renewal_interval_quantity,
			has_successful_payment = %[1]s.has_successful_payment OR excluded.has_successful_payment
		RETURNING has_successful_payment
	`, table))

	// RETURNING gives back the accumulated (sticky) flag in one round-trip, so
	// the in-memory sub reflects the persisted value — e.g. a past_due event on
	// a previously-active subscription still sees has_successful_payment = true.
	if err := r.db.QueryRowContext(
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
		sub.Status == "active",
	).Scan(&sub.HasSuccessfulPayment); err != nil {
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
	// Lenient grants access to on_trial/active/past_due/cancelled. Strict keeps
	// past_due only for subscriptions that have had a successful payment
	// (has_successful_payment), mirroring the past_due branch in IsActive.
	predicate := "status IN ('on_trial', 'active', 'past_due', 'cancelled')"
	if r.strictAccess {
		predicate = "(status IN ('on_trial', 'active', 'cancelled') " +
			"OR (status = 'past_due' AND has_successful_payment = TRUE))"
	}
	query := fmt.Sprintf(`
		SELECT user_id, product_id
		FROM %s
		WHERE product_id IS NOT NULL
		  AND %s
		ORDER BY user_id, updated_at DESC
	`, table, predicate)

	rows, err := r.db.QueryContext(ctx, query)
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
