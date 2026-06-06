# Token Leaderboard

Last updated: 2026-06-06

Current custom branch: `image-capability`

Current upstream base: `0.1.134`

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
- Administrators can remove a user from the public leaderboard without banning future participation.
- Administrators can ban a user from leaderboard participation; banned users cannot opt in again until unbanned.
- Public responses must not include email, username, database `user_id`, API Key, group, account, subscription, or raw identity fields.
- Public names are either a validated optional nickname or a stable masked display code such as `用户 #A83F`.
- Ranking tokens are `input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens + image_output_tokens`.
- Daily, weekly, monthly, and all-time public leaderboards only display users who are currently opted in and not banned.
- Daily, weekly, monthly, and all-time public leaderboards rank current participants by their usage in the selected window; the usage itself is not truncated by `opted_in_at`.
- Daily, weekly, and monthly current leaderboards are real-time and use the server configured timezone, so all users see the same global windows. Historical honors come from period snapshots.
- Manual and scheduled historical snapshots are internal raw usage snapshots. They do not filter by participation state during sync; visibility and honor calculation filter current participant state at read time.
- "Streak" means consecutive completed periods ranked first for the same window. The current unfinished period does not count toward streak.

## Data Model

- Migration: `backend/migrations/146_leaderboard.sql`
- `leaderboard_participants`
  - `user_id` primary key, references `users(id)`.
  - `is_opted_in`, `is_banned`, optional `display_name`, stable `display_code`, `opted_in_at`.
  - `display_name` is constrained to 1-32 trimmed characters when present.
- Migration: `backend/migrations/149_leaderboard_ban_status.sql`
  - Adds `leaderboard_participants.is_banned` for admin-controlled opt-in blocking.
  - Kept separate from migration 146 to avoid checksum drift on environments that already applied 146.
- `leaderboard_period_results`
  - Stores completed daily, weekly, and monthly raw usage snapshots.
  - Stores tokens, requests, rank, period start/end, and non-sensitive display snapshots.
  - Rank is not limited to Top 10 so later opt-in users can surface correct historical honors.
  - Unique indexes prevent duplicate rows for the same window/period/rank/user.
- Migration: `backend/migrations/150_leaderboard_full_snapshot_rank.sql`
  - Replaces the old Top 10 rank check with `rank >= 1`, allowing manual and scheduled snapshots to store full-period raw rankings.
- Existing `usage_logs` remains the source of current usage aggregation.

Migration conflict note: if upstream adds migrations after 146 or introduces its own leaderboard, re-number this migration during merge and compare semantics before keeping both features.

## Backend Touchpoints

- `backend/internal/service/leaderboard.go`
  - Opt-in DTOs, overview composition, nickname validation, privacy-safe public entries, streak calculation, and period snapshot worker.
- `backend/internal/repository/leaderboard_repo.go`
  - Raw SQL aggregation against `usage_logs`, participant visibility filtering, participant upsert, historical honor queries, and snapshot inserts.
- `backend/internal/handler/leaderboard_handler.go`
  - `GET /api/v1/leaderboard/overview`
  - `GET /api/v1/leaderboard/me`
  - `PUT /api/v1/leaderboard/me`
  - `POST /api/v1/admin/dashboard/leaderboard/backfill`
  - `POST /api/v1/admin/users/:id/leaderboard/remove`
  - `POST /api/v1/admin/users/:id/leaderboard/ban`
  - `POST /api/v1/admin/users/:id/leaderboard/unban`
- `backend/internal/server/routes/user.go`
  - Registers leaderboard routes behind user JWT auth.
- `backend/cmd/server/wire.go` and `backend/cmd/server/wire_gen.go`
  - Wires the leaderboard repository, service, handler, and 15-minute snapshot worker.

## Frontend Touchpoints

- `frontend/src/api/leaderboard.ts`
- `frontend/src/api/admin/dashboard.ts`
- `frontend/src/types/index.ts`
- `frontend/src/router/index.ts`
- `frontend/src/views/user/LeaderboardView.vue`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/UsersView.vue`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
  - The leaderboard view renders avatars, podium badges for the top 3, and a stronger ring/highlight for the current user row.

The page is a single user-facing page. It shows four panels: daily, weekly, monthly, all-time. Each panel shows Top 10 plus "my rank" when the current user has opted in.

The admin dashboard includes a compact manual backfill control for historical leaderboard snapshots. Admins choose a start date and trigger `POST /api/v1/admin/dashboard/leaderboard/backfill` with:

```json
{ "start": "2026-06-01" }
```

The response returns `start_time`, `end_time`, `period_count`, and `inserted_rows`. The backfill only snapshots completed daily, weekly, and monthly periods. It does not snapshot the current unfinished period. Re-running the same range deletes and rebuilds each affected period snapshot so old snapshot data can be corrected after ranking logic changes.

## Production Behavior

- No secrets or raw identities are exposed by public leaderboard DTOs.
- Opt-out does not delete `usage_logs`; it only hides public leaderboard participation and clears `opted_in_at`.
- Admin remove also clears public participation without deleting `usage_logs` or historical period snapshots.
- Admin ban clears public participation and blocks future self opt-in until admin unban.
- Rejoining makes the user visible again; current rankings use the selected window's historical usage while visibility remains controlled by current opt-in state.
- The snapshot worker runs every 15 minutes and snapshots recently completed daily, weekly, and monthly windows.
- Historical backfill is manual only. It must not run automatically on application startup.
- Manual backfill is duplicate-safe because each period is deleted and rebuilt in one transaction. Re-running the same date range corrects rows instead of double-counting.
- If the snapshot worker fails, current real-time leaderboards still work; historical honors may lag.

## Merge Checklist

- Check whether upstream changed `usage_logs` token columns or request success semantics.
- Check whether upstream added a public leaderboard or user privacy feature with overlapping tables or routes.
- Re-confirm public leaderboard responses still omit raw user identity fields.
- Re-confirm participation filtering is applied at public read/honor calculation time, not during snapshot sync.
- Re-confirm migration `150_leaderboard_full_snapshot_rank.sql` or an equivalent constraint update exists when snapshot code writes full ranks.
- Re-confirm banned participants are excluded from ranking queries and cannot self opt in.
- Re-confirm `image_output_tokens` remains included in token totals.
- Re-confirm `POST /api/v1/admin/dashboard/leaderboard/backfill` still snapshots only completed periods and remains duplicate-safe.
- Re-confirm the leaderboard snapshot worker interval remains 15 minutes unless intentionally changed.
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
- Admin remove hides a user from current public ranking without setting `is_banned`.
- Admin ban hides a user and prevents self opt-in; admin unban allows future opt-in but does not auto-rejoin.
- Admin dashboard manual backfill from a selected date succeeds and reports period count plus written rows.
- Re-running the same admin manual backfill rebuilds affected period rows and does not duplicate historical rows.
- Each section returns at most 10 public rows plus separate current-user rank.
- Public response JSON contains no `user_id`, `email`, `username`, `api_key`, `group_id`, `account_id`, or `subscription_id`.
- Historical champion count, top appearances, best rank, and streak update after completed period snapshots.

## Rollback Notes

- The leaderboard migrations add new tables and later relax the period-result rank constraint. Application rollback can normally leave these tables and the relaxed constraint in place.
- If a bug appears, hide the sidebar entry or roll back the application image. Existing billing, subscription quota, and usage logging paths are independent.
- If data cleanup is required, delete from `leaderboard_period_results` and `leaderboard_participants`; never delete `usage_logs` as part of leaderboard rollback.
