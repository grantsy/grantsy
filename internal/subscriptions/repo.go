package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/grantsy/grantsy/internal/infra/db"
)

type Subscription struct {
	ID                 int
	UserID             string
	CustomerID         int
	OrderID            int
	ProductID          int
	ProductName        string
	VariantID          int
	VariantName        string
	Status             string
	StatusFormatted    string
	CardBrand          string
	CardLastFour       string
	Cancelled          bool
	TrialEndsAt        *int64
	BillingAnchor      int
	SubscriptionItemID int
	RenewsAt           int64
	EndsAt             *int64
	CreatedAt          int64
	UpdatedAt          int64
}

// IsActive returns true if the subscription grants access.
func (s *Subscription) IsActive() bool {
	switch s.Status {
	case "on_trial", "active", "past_due", "cancelled":
		return true
	default:
		return false
	}
}

type Repo struct {
	db *db.DB
}

func NewRepo(database *db.DB) *Repo {
	return &Repo{db: database}
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
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
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
			updated_at = excluded.updated_at
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
			created_at, updated_at
		FROM %s
		WHERE user_id = $1
	`, table))

	var sub Subscription
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&sub.ID, &sub.UserID, &sub.CustomerID, &sub.OrderID, &sub.ProductID, &sub.ProductName,
		&sub.VariantID, &sub.VariantName, &sub.Status, &sub.StatusFormatted,
		&sub.CardBrand, &sub.CardLastFour, &sub.Cancelled, &sub.TrialEndsAt,
		&sub.BillingAnchor, &sub.SubscriptionItemID, &sub.RenewsAt, &sub.EndsAt,
		&sub.CreatedAt, &sub.UpdatedAt,
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
	query := fmt.Sprintf(`
		SELECT user_id, product_id
		FROM %s
		WHERE product_id IS NOT NULL
		  AND status IN ('on_trial', 'active', 'past_due', 'cancelled')
	`, table)

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
		result[userID] = productID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("subscriptions: rows error: %w", err)
	}

	return result, nil
}
