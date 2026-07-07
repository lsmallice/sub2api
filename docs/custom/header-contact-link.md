# Header Contact Link

Last updated: 2026-06-18

Current custom branch: `image-capability`

Current upstream base: `0.1.146`

## Purpose

Show a compact configurable contact entry in the desktop application header, in the empty space between the page title and the right-side controls. Operators can use it for Telegram groups, QQ groups, support rooms, or other external contact links without hard-coding site-specific information.

## Core Rules

- The feature is optional and hidden by default.
- The header link is shown only when both `header_contact_label` and `header_contact_url` are configured.
- `contact_info` remains the legacy plain-text support contact used by profile/redeem/dropdown surfaces; it is not reused for the header link.
- The URL must be `http://` or `https://`; invalid or non-HTTP(S) values are normalized to empty and therefore hidden.
- External navigation uses a new browser context with `target="_blank"` and `rel="noopener noreferrer"`.
- The desktop header link must stay compact and must not increase header height. Mobile header keeps it hidden to avoid crowding core navigation controls.

## Data Model

No migration is required. The feature uses the existing `settings` key-value table.

Settings keys:

- `header_contact_label`: display text, for example `TG 群 / QQ 群 123456`.
- `header_contact_url`: external URL, for example `https://t.me/example` or `https://qm.qq.com/q/...`.

Default initialization writes both keys as empty strings for new deployments.

## Backend Touchpoints

- `backend/internal/service/domain_constants.go`
  - Defines `SettingKeyHeaderContactLabel`.
  - Defines `SettingKeyHeaderContactURL`.
- `backend/internal/service/settings_view.go`
  - Adds the fields to `SystemSettings` and `PublicSettings`.
- `backend/internal/service/setting_service.go`
  - Loads the keys in `GetPublicSettings`.
  - Normalizes the URL through `normalizePublicHTTPURL`.
  - Includes the fields in SSR injection through `PublicSettingsInjectionPayload`.
  - Persists values in the system settings update path.
  - Initializes default empty keys in `InitializeDefaultSettings`.
- `backend/internal/handler/dto/settings.go`
  - Exposes the fields in admin and public settings DTOs.
- `backend/internal/handler/setting_handler.go`
  - Returns the fields from `/api/v1/settings/public`.
- `backend/internal/handler/admin/setting_handler.go`
  - Accepts, returns, and audits changes to the two fields.

## Frontend Touchpoints

- `frontend/src/components/layout/AppHeader.vue`
  - Renders the compact desktop header link when both fields exist.
- `frontend/src/views/admin/SettingsView.vue`
  - Adds two inputs under site settings.
  - Clears invalid non-HTTP(S) URLs on save.
- `frontend/src/types/index.ts`
  - Adds optional fields to `PublicSettings`.
- `frontend/src/api/admin/settings.ts`
  - Adds admin settings response/update typings.
- `frontend/src/stores/app.ts`
  - Keeps fallback public settings shape compatible when cached values are synthesized.
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
  - Adds admin UI copy.

## Production Behavior

- Existing deployments without the two keys continue to work; the header entry is hidden.
- Admins can configure the values from the site settings page.
- Because the URL is public settings data, do not place secrets or private invitation tokens that should not be visible to authenticated users.

## Merge Checklist

- Preserve the two settings keys when upstream changes the settings service or public settings DTOs.
- Preserve SSR injection parity with `dto.PublicSettings`; otherwise the header link can disappear on first page load until the async settings request completes.
- Preserve the distinction between plain `contact_info` and linked `header_contact_*`.
- Re-check `AppHeader.vue` layout after upstream header/sidebar refactors so the contact chip does not overlap title, docs, language, subscription, balance, or user menu controls.
- Keep URL normalization restrictive unless there is a deliberate product decision to support non-HTTP schemes.

## Regression Test Plan

Backend:

```bash
cd backend
go test ./internal/service ./internal/handler ./cmd/server
```

Frontend:

```bash
cd frontend
corepack pnpm@9 run typecheck
corepack pnpm@9 run build
```

Manual:

- With empty `header_contact_label` and `header_contact_url`, confirm no header contact chip is visible.
- Configure only one field and confirm the chip remains hidden.
- Configure both fields with an HTTPS URL and confirm the chip appears in the desktop header and opens externally.
- Configure an invalid URL and confirm saving clears/hides it.
- Confirm mobile header remains uncluttered.

## Rollback Notes

Application rollback is enough. The settings keys are additive and harmless if the older application ignores them. No database rollback is required.
