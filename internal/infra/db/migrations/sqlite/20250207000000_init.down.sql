-- GoQite job queue

DROP INDEX IF EXISTS goqite_queue_priority_created_idx;
DROP TABLE IF EXISTS goqite;
DROP TRIGGER IF EXISTS goqite_updated_timestamp;

-- LemonSqueezy subscriptions

DROP INDEX IF EXISTS idx_subscriptions_lemonsqueezy_status;
DROP TABLE IF EXISTS subscriptions_lemonsqueezy;
