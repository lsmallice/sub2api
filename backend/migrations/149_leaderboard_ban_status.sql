-- Add admin-controlled leaderboard participation ban state.
-- Separate migration avoids modifying the already-created leaderboard migration.

ALTER TABLE leaderboard_participants
    ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_leaderboard_participants_banned
    ON leaderboard_participants (is_banned, updated_at DESC)
    WHERE is_banned = true;
