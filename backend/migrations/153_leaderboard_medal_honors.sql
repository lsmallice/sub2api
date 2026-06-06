-- Extend materialized leaderboard honors with silver/bronze medal counts and runner-up streaks.

ALTER TABLE leaderboard_user_honors
    ADD COLUMN IF NOT EXISTS runner_up_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS third_place_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS longest_runner_up_streak INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS perennial_runner_up BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE leaderboard_user_honors
    DROP CONSTRAINT IF EXISTS leaderboard_user_honors_non_negative_check;

ALTER TABLE leaderboard_user_honors
    ADD CONSTRAINT leaderboard_user_honors_non_negative_check
        CHECK (
            top_appearances >= 0
            AND champion_count >= 0
            AND runner_up_count >= 0
            AND third_place_count >= 0
            AND best_rank >= 0
            AND current_streak >= 0
            AND longest_runner_up_streak >= 0
        );

CREATE INDEX IF NOT EXISTS idx_leaderboard_user_honors_window_runner_up
    ON leaderboard_user_honors (period_window, longest_runner_up_streak DESC, runner_up_count DESC);
