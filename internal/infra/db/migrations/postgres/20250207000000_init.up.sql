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

create extension if not exists pgcrypto;

create function update_timestamp()
returns trigger as $$
begin
   new.updated = now();
   return new;
end;
$$ language plpgsql;

create table goqite (
  id text primary key default ('m_' || encode(gen_random_bytes(16), 'hex')),
  created timestamptz not null default now(),
  updated timestamptz not null default now(),
  queue text not null,
  body bytea not null,
  timeout timestamptz not null default now(),
  received integer not null default 0,
  priority integer not null default 0
);

create trigger goqite_updated_timestamp
before update on goqite
for each row execute procedure update_timestamp();

create index goqite_queue_priority_created_idx on goqite (queue, priority desc, created);
