DROP INDEX IF EXISTS idx_{ns}subscriptions_lemonsqueezy_user_active;

ALTER TABLE {ns}subscriptions_lemonsqueezy
  DROP COLUMN refunded_at;

CREATE UNIQUE INDEX IF NOT EXISTS idx_{ns}subscriptions_lemonsqueezy_user_active
ON {ns}subscriptions_lemonsqueezy(user_id)
WHERE status IN ('on_trial', 'active', 'past_due', 'cancelled');
