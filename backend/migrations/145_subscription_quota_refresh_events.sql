-- Audit table for paid early subscription quota refreshes.
-- No foreign keys are used because revoked subscriptions may be soft/hard deleted
-- while the validity-for-quota exchange history should remain inspectable.
CREATE TABLE IF NOT EXISTS subscription_quota_refresh_events (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    actor_type VARCHAR(16) NOT NULL CHECK (actor_type IN ('user', 'admin')),
    actor_id BIGINT,
    quota_window VARCHAR(16) NOT NULL CHECK (quota_window IN ('daily', 'weekly', 'monthly')),
    deducted_seconds BIGINT NOT NULL CHECK (deducted_seconds > 0),
    old_expires_at TIMESTAMPTZ NOT NULL,
    new_expires_at TIMESTAMPTZ NOT NULL,
    old_window_start TIMESTAMPTZ,
    new_window_start TIMESTAMPTZ NOT NULL,
    old_usage_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    limit_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    idempotency_key_hash VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscription_quota_refresh_events_subscription_id
    ON subscription_quota_refresh_events(subscription_id);

CREATE INDEX IF NOT EXISTS idx_subscription_quota_refresh_events_user_id_created_at
    ON subscription_quota_refresh_events(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_subscription_quota_refresh_events_group_id_created_at
    ON subscription_quota_refresh_events(group_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_subscription_quota_refresh_events_idempotency_key_hash
    ON subscription_quota_refresh_events(idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL AND idempotency_key_hash <> '';
