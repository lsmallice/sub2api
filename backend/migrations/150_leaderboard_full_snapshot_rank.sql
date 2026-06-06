-- Allow leaderboard period snapshots to keep full historical ranks.
-- Public Top 10 filtering is handled at read time.

ALTER TABLE leaderboard_period_results
    DROP CONSTRAINT IF EXISTS leaderboard_period_results_rank_check;

ALTER TABLE leaderboard_period_results
    ADD CONSTRAINT leaderboard_period_results_rank_check
        CHECK (rank >= 1);
