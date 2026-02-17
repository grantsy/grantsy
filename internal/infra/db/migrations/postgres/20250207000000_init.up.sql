-- LemonSqueezy subscriptions
CREATE TABLE IF NOT EXISTS subscriptions_lemonsqueezy (
    id                   INTEGER PRIMARY KEY,
    user_id              TEXT NOT NULL,
    customer_id          INTEGER NOT NULL DEFAULT 0,
    order_id             INTEGER NOT NULL DEFAULT 0,
    product_id           INTEGER NOT NULL DEFAULT 0,
    product_name         TEXT NOT NULL DEFAULT '',
    variant_id           INTEGER NOT NULL DEFAULT 0,
    variant_name         TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT '',
    status_formatted     TEXT NOT NULL DEFAULT '',
    card_brand           TEXT NOT NULL DEFAULT '',
    card_last_four       TEXT NOT NULL DEFAULT '',
    cancelled            BOOLEAN NOT NULL DEFAULT FALSE,
    trial_ends_at        INTEGER,
    billing_anchor       INTEGER NOT NULL DEFAULT 0,
    subscription_item_id INTEGER NOT NULL DEFAULT 0,
    renews_at            INTEGER NOT NULL DEFAULT 0,
    ends_at              INTEGER,
    created_at           INTEGER NOT NULL DEFAULT 0,
    updated_at           INTEGER NOT NULL DEFAULT 0,
    price_id             INTEGER NOT NULL DEFAULT 0,
    unit_price           INTEGER NOT NULL DEFAULT 0,
    renewal_interval_unit     TEXT NOT NULL DEFAULT '',
    renewal_interval_quantity INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_lemonsqueezy_status ON subscriptions_lemonsqueezy(status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_lemonsqueezy_user_active
ON subscriptions_lemonsqueezy(user_id)
WHERE status IN ('on_trial', 'active', 'past_due', 'cancelled');