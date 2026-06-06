-- Add daily usage aggregates for fast leaderboard reads.
-- Snapshot/backfill writes raw usage without participant filtering; public reads filter visibility.

CREATE TABLE IF NOT EXISTS leaderboard_usage_daily (
    usage_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tokens BIGINT NOT NULL DEFAULT 0,
    requests BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (usage_date, user_id),
    CONSTRAINT leaderboard_usage_daily_tokens_check CHECK (tokens >= 0),
    CONSTRAINT leaderboard_usage_daily_requests_check CHECK (requests >= 0)
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_usage_daily_user_date
    ON leaderboard_usage_daily (user_id, usage_date DESC);

CREATE INDEX IF NOT EXISTS idx_leaderboard_usage_daily_date_tokens
    ON leaderboard_usage_daily (usage_date, tokens DESC, requests DESC);
