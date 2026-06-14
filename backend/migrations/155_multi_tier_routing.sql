-- Multi-tier group routing.
--
-- This migration is additive and keeps legacy single-rate groups unchanged.
-- groups.rate_multiplier remains the fallback when no active tier exists.

CREATE TABLE IF NOT EXISTS group_rate_tiers (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    tier_key VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    priority INT NOT NULL DEFAULT 50,
    enabled BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    fallback_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT group_rate_tiers_rate_multiplier_nonnegative CHECK (rate_multiplier >= 0),
    CONSTRAINT group_rate_tiers_tier_key_not_blank CHECK (length(trim(tier_key)) > 0),
    UNIQUE (group_id, tier_key)
);

CREATE INDEX IF NOT EXISTS idx_group_rate_tiers_group_enabled_priority
    ON group_rate_tiers (group_id, enabled, priority, id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_group_rate_tiers_one_default
    ON group_rate_tiers (group_id)
    WHERE is_default = true;

COMMENT ON TABLE group_rate_tiers IS 'Custom feature: service tiers within a group, each with its own user billing multiplier.';
COMMENT ON COLUMN group_rate_tiers.tier_key IS 'Stable tier key such as pro, plus, or pro2.';
COMMENT ON COLUMN group_rate_tiers.rate_multiplier IS 'User billing multiplier used when this tier is actually selected.';
COMMENT ON COLUMN group_rate_tiers.fallback_policy IS 'Optional tier-local fallback/degrade policy. API key policy can override user preference.';

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS service_tier_key VARCHAR(50) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_accounts_service_tier_key
    ON accounts (service_tier_key)
    WHERE deleted_at IS NULL AND service_tier_key <> '';

COMMENT ON COLUMN accounts.service_tier_key IS 'Custom feature: service tier this account serves, such as pro, plus, or pro2. Empty means legacy/unclassified.';

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS preferred_tier_key VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tier_fallback_enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS tier_fallback_policy JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_api_keys_preferred_tier_key
    ON api_keys (preferred_tier_key)
    WHERE deleted_at IS NULL AND preferred_tier_key <> '';

COMMENT ON COLUMN api_keys.preferred_tier_key IS 'Custom feature: preferred default service tier for this key.';
COMMENT ON COLUMN api_keys.tier_fallback_enabled IS 'Custom feature: whether this key can fall back to another group tier.';
COMMENT ON COLUMN api_keys.tier_fallback_policy IS 'Custom feature: user/key fallback policy, e.g. ordered fallback tiers and future TTFT thresholds.';

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS requested_tier_key VARCHAR(50),
    ADD COLUMN IF NOT EXISTS actual_tier_key VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_usage_logs_actual_tier_created_at
    ON usage_logs (actual_tier_key, created_at DESC)
    WHERE actual_tier_key IS NOT NULL AND actual_tier_key <> '';

COMMENT ON COLUMN usage_logs.requested_tier_key IS 'Custom feature: preferred/default tier requested before fallback.';
COMMENT ON COLUMN usage_logs.actual_tier_key IS 'Custom feature: actual tier selected for routing and billing.';

CREATE TABLE IF NOT EXISTS group_tier_health_events (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    tier_key VARCHAR(50) NOT NULL,
    model_key VARCHAR(100) NOT NULL DEFAULT '',
    capability VARCHAR(50) NOT NULL DEFAULT '',
    old_state VARCHAR(20) NOT NULL DEFAULT '',
    new_state VARCHAR(20) NOT NULL,
    reason VARCHAR(100) NOT NULL DEFAULT '',
    observed_ttft_ms INT,
    observed_error_rate DECIMAL(10,6),
    sample_count INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_group_tier_health_events_group_tier_created
    ON group_tier_health_events (group_id, tier_key, created_at DESC);

COMMENT ON TABLE group_tier_health_events IS 'Custom feature audit log for future tier degrade/probe/recovery state transitions.';
