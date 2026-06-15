# Multi-Tier Routing

Last updated: 2026-06-14

Current custom branch: `image-capability`

Current upstream base: `0.1.135`

## Purpose

Multi-tier routing lets one group expose multiple service tiers with independent user billing multipliers, such as `PRO` at `2x` and `Plus` at `1x`. Accounts are tagged with a matching service tier such as `pro`, `plus`, or `pro2`, and users or API keys can choose a preferred default tier.

The full target behavior is a closed loop:

- route to the user's preferred tier by default,
- fall back to another tier when the preferred tier is unavailable or degraded,
- let users configure fallback policy,
- use first-token latency and availability signals to degrade a tier,
- probe and restore a degraded tier automatically,
- bill and audit according to the tier actually used.

## Core Rules

- Existing `groups.rate_multiplier` remains the single-tier compatibility fallback.
- A group with no configured active `group_rate_tiers` behaves exactly as before.
- `group_rate_tiers.rate_multiplier` is the user-facing billing multiplier for requests routed through that tier.
- `accounts.service_tier_key` declares which tier an account can serve. Empty means legacy/unclassified and is only eligible when no tier filtering is requested.
- API keys can store a preferred tier and fallback policy. Empty preferred tier means use the group's default tier, then the legacy group multiplier if no tier exists.
- Usage logs must record both requested and actual tier when multi-tier routing is used.
- Usage list APIs and exports expose both requested and actual tier, so fallback can be audited after the request.
- Billing uses the actual tier multiplier, not merely the requested/default tier multiplier.
- Image-generation, endpoint capability, compact, transport, model mapping, privacy, account health, and existing group membership checks still apply after tier filtering.
- Tier fallback must never bypass group permissions or account capability checks.
- Tier health only activates when the API key fallback policy explicitly enables health/degrade behavior or sets a latency/error threshold. Existing keys with no thresholds keep static tier routing only.
- Degraded tier state is keyed by `group_id + tier_key + requested model + capability/transport` so one bad model or mode does not poison every use of the tier.
- A degraded tier is skipped until cooldown expires. After cooldown, it enters probe mode; enough successful probe samples restore it to healthy.
- Slow first-token samples and upstream failover errors can degrade a tier. User/request validation errors, such as image request parameter errors returned by upstream, should not degrade the tier.

## Data Model

Migration anchor:

- `backend/migrations/155_multi_tier_routing.sql`

Tables:

- `group_rate_tiers`
  - `group_id`
  - `tier_key`
  - `display_name`
  - `rate_multiplier`
  - `priority` (internal ordering value; admin UI saves it from the visible tier order)
  - `enabled`
  - `is_default`
  - `fallback_policy`
  - timestamps
- `group_tier_health_events`
  - audit trail for degrade/probe/recover state transitions.

Columns:

- `accounts.service_tier_key`
- `api_keys.preferred_tier_key`
- `api_keys.tier_fallback_enabled`
- `api_keys.tier_fallback_policy`
- `usage_logs.requested_tier_key`
- `usage_logs.actual_tier_key`

The current implementation uses these fields for static routing, actual-tier billing, usage logging, and a process-local hot health state. `group_tier_health_events` records state transitions for audit. The hot state is intentionally ephemeral; process restart returns tiers to healthy unless account-level scheduler state also blocks them.

## Backend Touchpoints

Expected anchors:

- `backend/internal/service/multi_tier_routing.go`
- `backend/internal/repository/group_rate_tier_repo.go`
- `backend/internal/service/account.go`
- `backend/internal/service/api_key.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/handler/api_key_handler.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`

Static routing phase:

1. Resolve effective tier candidates from API key preference and group default tier.
2. Select accounts only from the current candidate tier.
3. If no account is available and fallback is enabled, retry selection using the next tier candidate.
4. Pass requested and actual tier into usage recording.
5. Apply the actual tier's multiplier to user-facing cost.

Usage observability:

1. `usage_logs.requested_tier_key` stores the policy/preference tier that the request attempted first.
2. `usage_logs.actual_tier_key` stores the tier that actually served the request.
3. Batched, best-effort, and single-row usage log insert paths must all include both columns.
4. User and admin usage DTOs expose both fields.
5. User CSV export and admin XLSX export include both requested and actual tier.

Health phase:

1. Parse API key `tier_fallback_policy`.
2. Enable health handling when one of these is true:
   - `degrade_enabled=true`
   - `health_enabled=true`
   - `first_token_threshold_ms` / `ttft_threshold_ms` / `first_token_timeout_ms` / `ttft_timeout_ms` is set
   - `degrade_after_errors` / `error_threshold` / `error_sample_threshold` is set
3. Supported policy knobs:
   - fallback order: `fallback_order`, `fallback_tiers`, `order`, or `tiers`
   - TTFT threshold: `first_token_threshold_ms`, `ttft_threshold_ms`, `first_token_timeout_ms`, or `ttft_timeout_ms`
   - slow-sample threshold: `degrade_after_slow_samples`, `slow_sample_threshold`, or `ttft_sample_threshold`; default `1`
   - error threshold: `degrade_after_errors`, `error_threshold`, or `error_sample_threshold`; default `2`
   - cooldown: `cooldown_seconds`, `recovery_cooldown_seconds`, or `probe_after_seconds`; default `300`
   - recovery successes: `recovery_successes`, `probe_successes`, or `recover_after_successes`; default `2`
4. Mark a tier `degraded` when the configured slow/error threshold is exceeded.
5. Skip `degraded` tiers and try fallback candidates when fallback is enabled.
6. After cooldown, allow that tier as `probing`.
7. Restore to `healthy` after enough successful probe samples.
8. Return to `degraded` if a probe is slow or fails.

## Frontend Touchpoints

Expected anchors:

- `frontend/src/types/index.ts`
- `frontend/src/api/admin/groups.ts`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/api/keys.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/views/user/UsageView.vue`
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

User UI should show the preferred tier, whether fallback is enabled, and usage rows should show the actual tier used. Admin UI should manage group tiers and account tier tags.

Admin group UI rules:

- The group list must show an OpenAI tier summary column so configured tiers are visible without opening the modal.
- The tier summary should show the default tier first, then enabled tiers, then disabled tiers; it should stay compact and link back to tier configuration.
- The tier editor should not expose raw `priority` as a hand-entered number. The visible card order is the admin-facing control, and save payloads derive `priority=(index+1)*10`.
- `priority` is tier candidate order only. It is not the account scheduler priority. Multiple accounts in the same tier still use account priority, load, and LRU scheduling.
- Fallback policy settings remain advanced tier settings: fallback order, first-token threshold, error threshold, cooldown, and recovery successes.

## Production Behavior

- With no tier rows, production behavior is unchanged.
- With tier rows but no API key preference, the group's default tier is used.
- If the preferred/default tier has no eligible account:
  - fallback enabled: try fallback candidates in policy order,
  - fallback disabled: fail with a clear tier-unavailable error.
- `fallback_order` stores only fallback candidates after the effective primary tier. If the API key chooses "use group default", the effective primary tier is the group's default tier, and that tier must not appear in the stored or displayed fallback order.
- If the preferred/default tier is degraded:
  - fallback enabled: skip it until cooldown or probe recovery,
  - fallback disabled: fail when no allowed candidate remains.
- Usage analytics should be able to distinguish requested vs actual tier.

## Merge Checklist

- Confirm migration number does not conflict with upstream or other custom migrations.
- Confirm new nullable columns are still selected and inserted by `usage_log_repo.go`.
- Confirm account DTO/admin forms preserve `service_tier_key`.
- Confirm API key DTO/user forms preserve preferred tier and fallback policy.
- Confirm admin group list still returns and renders `rate_tiers` summaries.
- Confirm tier editor still hides raw priority and derives it from card order.
- Confirm scheduler still respects image capability, compact, privacy, model mapping, transport, and sticky-session checks after tier filtering.
- Confirm no code path bills with the requested tier when a fallback tier was actually used.
- Confirm no unconfigured group changes behavior.
- Confirm handler success/failure paths still report tier health after upstream failover refactors.
- Confirm `group_tier_health_events` remains additive and audit-only; routing must not depend on querying it per request.

## Regression Test Plan

- Group with no `group_rate_tiers` routes and bills exactly as before.
- Group with `pro=2x` and `plus=1x` routes preferred `pro` keys only to `service_tier_key=pro` accounts.
- Fallback-enabled key falls from `pro` to `plus` when no `pro` account is available.
- Fallback-disabled key fails when preferred tier has no account.
- Usage logs store `requested_tier_key=pro`, `actual_tier_key=plus` after fallback.
- User/admin usage pages and exports show requested tier vs actual tier.
- Cost uses the actual tier multiplier.
- Image-generation routing still requires image-capable accounts inside the selected tier.
- Sticky session does not keep a request on an account from the wrong tier.
- Future health phase: TTFT degradation, cooldown, probe, and recovery are tested independently.
- TTFT degradation skips a preferred tier and falls back to the next tier.
- Error threshold degradation skips a preferred tier and falls back to the next tier.
- Cooldown promotes a degraded tier to probe mode.
- Enough probe successes restore the tier to healthy.

Suggested commands:

```bash
go test ./internal/service -run 'TestOpenAIGatewayService_SelectAccountWithTierRouting|TestOpenAIGatewayServiceRecordUsage'
go test ./internal/repository -run 'TestMultiTier|TestUsageLog|TestAPIKey|TestAccount'
go test ./internal/handler -run 'TestAPIKey|TestAccount|TestGroup'
corepack pnpm@9 run typecheck
```

## Rollback Notes

The migration is additive. Application rollback can leave the new tables and nullable columns in place. If tier routing causes issues, disable it by removing or disabling `group_rate_tiers` rows; groups without active tiers fall back to legacy `groups.rate_multiplier`.
