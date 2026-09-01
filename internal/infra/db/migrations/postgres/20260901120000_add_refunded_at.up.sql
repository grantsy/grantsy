ALTER TABLE subscriptions_lemonsqueezy
  ADD COLUMN refunded_at INTEGER;

-- A refunded subscription is no longer active, so it must not occupy the
-- one-active-subscription-per-user slot: re-purchasing creates a row with a
-- new LemonSqueezy subscription ID that would otherwise collide here.
DROP INDEX IF EXISTS idx_subscriptions_lemonsqueezy_user_active;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_lemonsqueezy_user_active
ON subscriptions_lemonsqueezy(user_id)
WHERE status IN ('on_trial', 'active', 'past_due', 'cancelled')
  AND refunded_at IS NULL;
