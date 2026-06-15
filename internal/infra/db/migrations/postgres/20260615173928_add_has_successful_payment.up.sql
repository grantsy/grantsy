ALTER TABLE subscriptions_lemonsqueezy
  ADD COLUMN has_successful_payment BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill only active: the single status that definitively implies a
-- successful payment. All other statuses stay FALSE.
UPDATE subscriptions_lemonsqueezy
  SET has_successful_payment = TRUE
  WHERE status = 'active';
