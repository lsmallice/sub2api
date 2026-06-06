-- Store materialized leaderboard honor stats for fast page reads.
-- Rebuilt after period snapshots, manual backfill, and participant visibility changes.

CREATE TABLE IF NOT EXISTS leaderboard_user_honors (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period_window VARCHAR(16) NOT NULL,
    top_appearances INTEGER NOT NULL DEFAULT 0,
    champion_count INTEGER NOT NULL DEFAULT 0,
    best_rank INTEGER NOT NULL DEFAULT 0,
    current_streak INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, period_window),
    CONSTRAINT leaderboard_user_honors_window_check
        CHECK (period_window IN ('daily', 'weekly', 'monthly')),
    CONSTRAINT leaderboard_user_honors_non_negative_check
        CHECK (
            top_appearances >= 0
            AND champion_count >= 0
            AND best_rank >= 0
            AND current_streak >= 0
        )
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_user_honors_window_champions
    ON leaderboard_user_honors (period_window, champion_count DESC, current_streak DESC);
