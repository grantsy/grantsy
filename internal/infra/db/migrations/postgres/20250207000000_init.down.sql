-- GoQite job queue

DROP INDEX IF EXISTS goqite_queue_priority_created_idx;
DROP TABLE IF EXISTS goqite;
DROP FUNCTION IF EXISTS update_timestamp;

-- LemonSqueezy subscriptions

DROP INDEX IF EXISTS idx_subscriptions_lemonsqueezy_status;
DROP TABLE IF EXISTS subscriptions_lemonsqueezy;
