-- Add opt-in token leaderboard tables.
-- Public leaderboard output must be derived from these tables without exposing raw user identifiers.

CREATE TABLE IF NOT EXISTS leaderboard_participants (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    is_opted_in BOOLEAN NOT NULL DEFAULT false,
    display_name VARCHAR(32),
    display_code VARCHAR(16) NOT NULL UNIQUE,
    opted_in_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT leaderboard_participants_display_name_length
        CHECK (display_name IS NULL OR length(btrim(display_name)) BETWEEN 1 AND 32),
    CONSTRAINT leaderboard_participants_opted_in_at_check
        CHECK ((is_opted_in = false) OR opted_in_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_participants_opted_in
    ON leaderboard_participants (is_opted_in, opted_in_at)
    WHERE is_opted_in = true;

CREATE TABLE IF NOT EXISTS leaderboard_period_results (
    id BIGSERIAL PRIMARY KEY,
    period_window VARCHAR(16) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    rank INTEGER NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    display_name_snapshot VARCHAR(32),
    display_code_snapshot VARCHAR(16) NOT NULL,
    tokens BIGINT NOT NULL DEFAULT 0,
    requests BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT leaderboard_period_results_window_check
        CHECK (period_window IN ('daily', 'weekly', 'monthly')),
    CONSTRAINT leaderboard_period_results_rank_check
        CHECK (rank BETWEEN 1 AND 10),
    CONSTRAINT leaderboard_period_results_period_check
        CHECK (period_end > period_start)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_leaderboard_period_results_window_period_rank
    ON leaderboard_period_results (period_window, period_start, rank);

CREATE UNIQUE INDEX IF NOT EXISTS idx_leaderboard_period_results_window_period_user
    ON leaderboard_period_results (period_window, period_start, user_id);

CREATE INDEX IF NOT EXISTS idx_leaderboard_period_results_user_window_period
    ON leaderboard_period_results (user_id, period_window, period_start DESC);
