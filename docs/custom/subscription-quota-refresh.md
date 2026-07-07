# Subscription Quota Refresh

Last updated: 2026-06-17

Current custom branch: `image-capability`

Current upstream base: `0.1.146`

## Purpose

This overlay lets a subscription exhaust a daily, weekly, or monthly quota window and then refresh that window early by deducting remaining subscription validity. It is available to both users and admins, but it must stay distinct from the existing free admin quota reset flow.

The upstream `main` branch remains the source of truth. During upstream merges, preserve official subscription, quota, billing, and idempotency behavior first, then reapply this feature as a minimal overlay.

## Core Rules

- Only one refresh window may be selected per request: `daily`, `weekly`, or `monthly`.
- A refresh is allowed only when the selected quota window is already exhausted.
- The subscription must still be active and must still have enough validity left after deducting the current window's remaining time.
- Refresh deduction is dynamic: `deducted_duration = next_reset_at - now`.
- Daily, weekly, and monthly reset anchors still follow the existing rolling-window rules: daily `24h`, weekly `7 * 24h`, monthly `30 * 24h`.
- Monthly refresh still uses a rolling 30-day window, not a calendar-month rule.
- Paid refresh uses the exact current time as the new window start so the user receives a full fresh quota period.
- If the window has already reached its natural reset time, the feature must not deduct validity. It should report that natural reset is available or otherwise unnecessary.
- Every refresh request must carry `Idempotency-Key`.
- User and admin flows both write audit events, but the admin path records an admin actor and is still a paid refresh, not the free reset action.
- The feature can be disabled instantly with `SUBSCRIPTION_QUOTA_REFRESH_ENABLED=false`.

## Data Model

Migration:

- `backend/migrations/145_subscription_quota_refresh_events.sql`

New audit table:

- `subscription_quota_refresh_events`

Recorded fields:

- `subscription_id`
- `user_id`
- `group_id`
- `actor_type`
- `actor_id`
- `quota_window`
- `deducted_seconds`
- `old_expires_at`
- `new_expires_at`
- `old_window_start`
- `new_window_start`
- `old_usage_usd`
- `limit_usd`
- `idempotency_key_hash`
- `created_at`

Notes:

- This migration only adds an audit table.
- It does not change the existing subscription table shape.
- The table is append-only history, so ordinary app rollback can keep it in place.

## Backend Touchpoints

Service and capability logic:

- `backend/internal/service/subscription_quota_refresh.go`
- `backend/internal/service/subscription_service.go`
- `backend/internal/service/user_subscription.go`
- `backend/internal/service/user_subscription_port.go`

Repository:

- `backend/internal/repository/user_subscription_repo.go`

Config:

- `backend/internal/config/config.go`

Handlers and routes:

- `backend/internal/handler/subscription_handler.go`
- `backend/internal/handler/admin/subscription_handler.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/server/routes/admin.go`

DTOs and mappers:

- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`

Behavior:

- `RefreshSubscriptionQuota` validates ownership for user requests and records actor metadata for admin requests.
- Eligibility is computed once and reused for both API responses and the actual mutation path.
- `deducted_seconds` records the actual remaining-window time deducted for that refresh, not the full daily/weekly/monthly period.
- The repository locks the subscription row, clears the selected window usage, moves the selected window start to `now`, deducts `expires_at` by `next_reset_at - now`, and writes one audit row in the same transaction.
- After a successful refresh, subscription and billing caches are invalidated.
- Subscription list/detail DTOs expose `quota_refresh` metadata so the frontend does not reimplement eligibility rules.

Error codes:

- `QUOTA_REFRESH_DISABLED`
- `QUOTA_REFRESH_INVALID_WINDOW`
- `QUOTA_REFRESH_LIMIT_NOT_CONFIGURED`
- `QUOTA_REFRESH_NOT_EXHAUSTED`
- `QUOTA_REFRESH_INSUFFICIENT_VALIDITY`
- `QUOTA_REFRESH_NATURAL_RESET_AVAILABLE`
- `QUOTA_REFRESH_WINDOW_NOT_ACTIVE`
- `SUBSCRIPTION_NOT_ACTIVE`
- `IDEMPOTENCY_KEY_REQUIRED`

## Frontend Touchpoints

API:

- `frontend/src/api/subscriptions.ts`
- `frontend/src/api/admin/subscriptions.ts`

Types:

- `frontend/src/types/index.ts`

Views:

- `frontend/src/views/user/SubscriptionsView.vue`
- `frontend/src/views/admin/SubscriptionsView.vue`

I18n:

- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Behavior:

- User subscription pages show per-window refresh eligibility and short reasons.
- Admin subscription pages keep the free reset action separate from the paid validity deduction action.
- Both flows generate one `Idempotency-Key` per confirmation to prevent accidental double spending.

## Production Behavior

- The feature is controlled by `SUBSCRIPTION_QUOTA_REFRESH_ENABLED`.
- The public DTO already carries the eligibility summary, so the frontend should not guess.
- If a refresh is not eligible, the backend returns the specific reason instead of letting the frontend infer it.
- If this feature is shipped together with a migration, take a normal safety backup if your release process requires one, but ordinary app rollback should not need PostgreSQL restore because the schema change is additive.

## Merge Checklist

1. Read this document before resolving upstream conflicts.
2. Preserve upstream subscription quota semantics first, then reapply this overlay.
3. Search for the custom anchors:

```bash
rg -n "RefreshSubscriptionQuota|quota_refresh|QUOTA_REFRESH_|subscription_quota_refresh" backend frontend
rg -n "SUBSCRIPTION_QUOTA_REFRESH_ENABLED" backend frontend
rg -n "subscription_quota_refresh_events" backend
```

4. Verify the DTO summary still matches the backend eligibility logic.
5. Verify the free admin quota reset flow is still separate.
6. Verify the idempotency requirement remains enforced.
7. Verify the migration is still present or has been safely renumbered.
8. Update this document if the API, eligibility rules, audit fields, or rollback behavior change.

## Regression Test Plan

Backend tests:

```bash
go test ./internal/service -run 'TestRefreshSubscriptionQuota|TestAdminResetQuota'
go test ./internal/handler -run 'Test'
go test ./internal/repository -run 'Test'
```

Frontend build:

```bash
cd frontend
corepack pnpm@9 run build
```

Manual checks:

- Daily, weekly, and monthly refresh each deduct only the remaining time before natural reset.
- A daily window started 20 hours ago deducts about 4 hours.
- A weekly window started 6 days ago deducts about 1 day.
- A monthly window started 29 days ago deducts about 1 day.
- Refresh fails when the quota window is not exhausted.
- Refresh fails when remaining validity is too short.
- Refresh fails when the subscription is inactive or the limit is missing.
- If natural reset is already available, the feature does not deduct validity.
- The user can only refresh their own subscription.
- The admin paid refresh action stays distinct from the free reset action.
- Replaying the same `Idempotency-Key` does not deduct validity twice.

## Rollback Notes

- Fast rollback is application-level: restore the previous image/tag and restart only `sub2api`.
- If production trouble appears, the kill switch can disable the feature without affecting normal subscriptions or the admin free reset flow.
- The audit table can remain in place during normal rollback.
- Restore database backups only for confirmed corruption or an explicitly approved database rollback.
