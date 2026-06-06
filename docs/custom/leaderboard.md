# Token Leaderboard

Last updated: 2026-06-06

Current custom branch: `image-capability`

Current upstream base: `0.1.133`

## Purpose

This custom feature adds a voluntary, privacy-preserving Token leaderboard for end users. It ranks opted-in users by Token usage on one page with daily, weekly, monthly, and all-time sections. Each section shows only Top 10 plus the current user's own rank when the current user has opted in.

## Visual Notes

- Each row shows an avatar when available.
- The top three ranks use gold, silver, and bronze podium styling.
- Rank badges, left accent bars, and row borders change by placement for faster scanning.
- Missing avatars fall back to the public masked-name initial.
- Metric chips use progressively larger color bands rather than a single accent color.
- Token usage, request count, streak, and champion count each have their own low/mid/high/ultra tiers.
- Token and request tiers intentionally use wide thresholds so large accounts still show visible color progression instead of flattening too early.
- Streak and champion tiers also ramp up gradually, with the highest tier reserved for exceptional long-running performance.

## Core Rules

- Users are not ranked by default. They must explicitly opt in.
- Opting out immediately hides the user from public leaderboard responses.
- Public responses must not include email, username, database `user_id`, API Key, group, account, subscription, or raw identity fields.
- Public names are either a validated optional nickname or a stable masked display code such as `用户 #A83F`.
- Ranking tokens are `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens + image_output_tokens`.
- All leaderboard windows only count usage created after the user's current `opted_in_at`.
- Daily, weekly, and monthly current leaderboards are real-time and use the server configured timezone, so all users see the same global windows. Historical honors come from period snapshots.
- All-time leaderboard is cumulative usage after the current opt-in time.
- "Streak" means consecutive completed periods ranked first for the same window. The current unfinished period does not count toward streak.

## Data Model

- Migration: `backend/migrations/146_leaderboard.sql`
- `leaderboard_participants`
  - `user_id` primary key, references `users(id)`.
  - `is_opted_in`, optional `display_name`, stable `display_code`, `opted_in_at`.
  - `display_name` is constrained to 1-32 trimmed characters when present.
- `leaderboard_period_results`
  - Stores completed daily, weekly, and monthly Top 10 snapshots.
  - Stores masked display snapshots, tokens, requests, rank, period start/end.
  - Unique indexes prevent duplicate rows for the same window/period/rank/user.
- Existing `usage_logs` remains the source of current usage aggregation.

Migration conflict note: if upstream adds migrations after 146 or introduces its own leaderboard, re-number this migration during merge and compare semantics before keeping both features.

## Backend Touchpoints

- `backend/internal/service/leaderboard.go`
  - Opt-in DTOs, overview composition, nickname validation, privacy-safe public entries, streak calculation, and period snapshot worker.
- `backend/internal/repository/leaderboard_repo.go`
  - Raw SQL aggregation against `usage_logs`, opt-in filtering, participant upsert, historical honor queries, and snapshot inserts.
- `backend/internal/handler/leaderboard_handler.go`
  - `GET /api/v1/leaderboard/overview`
  - `GET /api/v1/leaderboard/me`
  - `PUT /api/v1/leaderboard/me`
- `backend/internal/server/routes/user.go`
  - Registers leaderboard routes behind user JWT auth.
- `backend/cmd/server/wire.go` and `backend/cmd/server/wire_gen.go`
  - Wires the leaderboard repository, service, handler, and hourly snapshot worker.

## Frontend Touchpoints

- `frontend/src/api/leaderboard.ts`
- `frontend/src/types/index.ts`
- `frontend/src/router/index.ts`
- `frontend/src/views/user/LeaderboardView.vue`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
  - The leaderboard view renders avatars, podium badges for the top 3, and a stronger ring/highlight for the current user row.

The page is a single user-facing page. It shows four panels: daily, weekly, monthly, all-time. Each panel shows Top 10 plus "my rank" when the current user has opted in.

## Production Behavior

- No secrets or raw identities are exposed by public leaderboard DTOs.
- Opt-out does not delete `usage_logs`; it only hides public leaderboard participation and clears `opted_in_at`.
- Rejoining starts all-time counting from the new `opted_in_at`.
- The snapshot worker runs hourly and backfills recently completed daily, weekly, and monthly windows. Duplicate workers are safe because snapshot inserts are idempotent.
- If the snapshot worker fails, current real-time leaderboards still work; historical honors may lag.

## Merge Checklist

- Check whether upstream changed `usage_logs` token columns or request success semantics.
- Check whether upstream added a public leaderboard or user privacy feature with overlapping tables or routes.
- Re-confirm public leaderboard responses still omit raw user identity fields.
- Re-confirm opt-in filtering is applied in every ranking query.
- Re-confirm `image_output_tokens` remains included in token totals.
- Re-run frontend route/sidebar checks if upstream refactors layout navigation.
- Re-run Wire or manually sync `wire_gen.go` if upstream regenerates dependency wiring.

## Regression Test Plan

Backend:

```bash
go test ./internal/service -run 'TestLeaderboard|TestCurrentLeaderboardStreak'
go test ./internal/repository -run 'TestLeaderboard'
go test ./internal/handler -run 'TestLeaderboard'
go test ./cmd/server -run 'TestProvideCleanup'
```

Frontend:

```bash
corepack pnpm@9 run typecheck
corepack pnpm@9 run build
```

Manual:

- Default user does not appear on any leaderboard.
- User opts in with no nickname and appears as a masked name.
- User opts in with nickname and appears with nickname only.
- Opted-out user disappears from daily, weekly, monthly, and all-time sections.
- Each section returns at most 10 public rows plus separate current-user rank.
- Public response JSON contains no `user_id`, `email`, `username`, `api_key`, `group_id`, `account_id`, or `subscription_id`.
- Historical champion count, top appearances, best rank, and streak update after completed period snapshots.

## Rollback Notes

- The migration only adds new tables. Application rollback can normally leave these tables in place.
- If a bug appears, hide the sidebar entry or roll back the application image. Existing billing, subscription quota, and usage logging paths are independent.
- If data cleanup is required, delete from `leaderboard_period_results` and `leaderboard_participants`; never delete `usage_logs` as part of leaderboard rollback.
