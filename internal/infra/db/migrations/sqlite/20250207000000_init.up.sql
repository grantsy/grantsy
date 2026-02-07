-- LemonSqueezy subscriptions
CREATE TABLE IF NOT EXISTS subscriptions_lemonsqueezy (
    id                   INTEGER PRIMARY KEY,
    user_id              TEXT NOT NULL UNIQUE,
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
    updated_at           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_lemonsqueezy_status ON subscriptions_lemonsqueezy(status);

-- GoQite job queue

create table goqite (
  id text primary key default ('m_' || lower(hex(randomblob(16)))),
  created text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  updated text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  queue text not null,
  body blob not null,
  timeout text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  received integer not null default 0,
  priority integer not null default 0
) strict;

create trigger goqite_updated_timestamp after update on goqite begin
  update goqite set updated = strftime('%Y-%m-%dT%H:%M:%fZ') where id = old.id;
end;

create index goqite_queue_priority_created_idx on goqite (queue, priority desc, created);
