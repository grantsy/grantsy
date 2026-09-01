DROP INDEX IF EXISTS idx_subscriptions_lemonsqueezy_user_active;

ALTER TABLE subscriptions_lemonsqueezy
  DROP COLUMN refunded_at;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_lemonsqueezy_user_active
ON subscriptions_lemonsqueezy(user_id)
WHERE status IN ('on_trial', 'active', 'past_due', 'cancelled');
