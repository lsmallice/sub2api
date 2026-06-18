# Infinite Canvas SSO And Image Key Binding

## Purpose

Infinite Canvas is integrated as a Sub2API tool site. Users enter Canvas through Sub2API SSO, choose only API Keys that can generate images, and all AI traffic is proxied server-side through Sub2API.

Sub2API remains the authority for authentication, billing, quotas, group permissions, account scheduling, usage logs, and risk controls.

## Runtime Model

- Sub2API creates a short-lived one-time SSO ticket for the current logged-in user.
- The browser is redirected to `canvas.base_url + /auth/sub2api/callback?ticket=...`.
- Infinite Canvas exchanges the ticket through an internal Sub2API endpoint and writes its own HttpOnly session cookie.
- Canvas frontend stores only the selected API Key ID. It never stores or receives the plaintext API Key.
- Canvas server-side proxy resolves the selected Key for each request using an internal service token, then forwards to Sub2API `/v1/...`.

Current ticket storage is process-local and intended for a single Sub2API container. If Sub2API is scaled to multiple replicas, move ticket state to Redis before enabling this in production.

## Sub2API Configuration

Environment variables:

- `CANVAS_ENABLED=true`
- `CANVAS_BASE_URL=https://canvas.smallice.xyz`
- `CANVAS_INTERNAL_SERVICE_TOKEN=<shared random secret>`
- `CANVAS_TICKET_TTL_SECONDS=120`

YAML equivalent:

```yaml
canvas:
  enabled: true
  base_url: "https://canvas.smallice.xyz"
  internal_service_token: "<shared random secret>"
  ticket_ttl_seconds: 120
```

For CA Docker, put these on the Sub2API container and put the same token on the Canvas container as `SUB2API_CANVAS_INTERNAL_TOKEN`.

## Canvas Configuration

Expected Canvas environment variables:

- `CANVAS_PUBLIC_BASE_URL=https://canvas.smallice.xyz`
- `CANVAS_SITE_NAME=<public site display name>`
- `SUB2API_INTERNAL_BASE_URL=http://sub2api-ca:8080`
- `SUB2API_PUBLIC_BASE_URL=https://api.smallice.xyz` or the public Sub2API origin used for redirects
- `SUB2API_CANVAS_INTERNAL_TOKEN=<same shared random secret>`
- `CANVAS_SESSION_SECRET=<random session signing secret>`

The Canvas frontend uses the fixed same-origin base URL `/api/sub2api`. It hides manual Base URL and plaintext API Key input.
`CANVAS_PUBLIC_BASE_URL` is used after SSO callback success or failure, so it must be the browser-visible Canvas origin rather than `0.0.0.0` or a container address.
`CANVAS_SITE_NAME` is shown in the Canvas configuration modal instead of exposing internal Sub2API naming.

## APIs

User-authenticated Sub2API APIs:

- `POST /api/v1/canvas/sso-ticket`
- `GET /api/v1/canvas/image-keys`

Internal Canvas-only Sub2API APIs:

- `POST /api/v1/internal/canvas/sso/exchange`
- `GET /api/v1/internal/canvas/users/:user_id/image-keys`
- `POST /api/v1/internal/canvas/api-key/resolve`

Internal APIs require `X-Canvas-Internal-Token` or `Authorization: Bearer <token>`.

Canvas proxy APIs:

- `GET /api/sub2api/v1/models`
- `POST /api/sub2api/v1/responses`
- `POST /api/sub2api/v1/images/generations`
- `POST /api/sub2api/v1/images/edits`

Video and audio routes are intentionally not included in v1.

## Image Key Eligibility

A Key is selectable only when all conditions are true:

- It belongs to the current user.
- It is active.
- It is not expired.
- Its configured quota is not exhausted.
- Its group allows image generation. Ungrouped keys keep existing Sub2API behavior.
- The relevant OpenAI account pool has at least one schedulable account with `supports_image_generation=true`.

The public Canvas key-list response includes only selection fields: `id`, `name`, `masked_key`, `group_name`, `expires_at`, `quota`, `quota_used`, and `image_eligible`.

It must not include email, username, user ID, plaintext API Key, account ID, or group ID.

## Code Anchors

Sub2API backend:

- `backend/internal/config/config.go`
- `backend/internal/service/canvas_service.go`
- `backend/internal/handler/canvas_handler.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/server/routes/canvas.go`
- `backend/internal/server/router.go`

Sub2API frontend:

- `frontend/src/api/canvas.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Infinite Canvas:

- `/Users/kevin/Documents/API/infinite-canvas/web/src/lib/sub2api-server.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/app/auth/sub2api/callback/route.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/app/api/sub2api/image-keys/route.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/app/api/sub2api/session/route.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/app/api/sub2api/v1/[...path]/route.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/stores/use-config-store.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/components/layout/app-config-modal.tsx`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/services/api/image.ts`

## Risks

- Duplicate/multi-replica SSO ticket state: current in-memory ticket store is safe only for one Sub2API process.
- Plaintext Key leakage: never return the resolved API Key to the browser; only the Canvas server-side proxy may receive it.
- Cross-user Key use: every resolve call must check `user_id + api_key_id` ownership and eligibility.
- Eligibility drift: group image permission and account `supports_image_generation` must remain aligned with the gateway's image-generation enforcement.
- Proxy expansion: adding video/audio endpoints later needs an explicit product and billing review.

## Regression

Sub2API:

- `go test ./internal/service ./internal/handler ./internal/server/routes`
- `corepack pnpm@9 run typecheck` from `frontend/`
- `corepack pnpm@9 run build` from `frontend/`

Infinite Canvas:

- `npm run build` from `/Users/kevin/Documents/API/infinite-canvas/web`
- `npx tsc --noEmit` currently has pre-existing errors outside this feature; ensure this feature does not add new errors.

Manual:

- Open Canvas from Sub2API sidebar and confirm SSO callback sets a Canvas session.
- Reuse the same ticket and confirm exchange fails.
- Confirm only image-eligible Keys appear.
- Confirm browser storage contains selected Key ID, not plaintext API Key.
- Confirm `/v1/models`, Responses, image generation, and image edits go through Sub2API and produce existing usage records.
