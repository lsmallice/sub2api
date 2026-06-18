# Infinite Canvas SSO And Image Key Binding

## Purpose

Infinite Canvas is integrated as a Sub2API tool site. Users enter Canvas through Sub2API SSO, choose only API Keys that can generate images, and all AI traffic is proxied server-side through Sub2API.

Sub2API remains the authority for authentication, billing, quotas, group permissions, account scheduling, usage logs, and risk controls.

## Runtime Model

- Sub2API creates a short-lived one-time SSO ticket for the current logged-in user.
- The browser is redirected to `canvas.base_url + /auth/sub2api/callback?ticket=...`.
- Infinite Canvas exchanges the ticket through an internal Sub2API endpoint and writes its own HttpOnly session cookie.
- If a user opens Canvas directly while unauthenticated, Canvas redirects to the public Sub2API web origin `/login?redirect=/canvas/launch`; after login, `/canvas/launch` creates a ticket and sends the user back to Canvas.
- The current light/dark theme is passed as a `theme=light|dark` callback parameter and applied to Canvas after SSO.
- Canvas frontend stores only the selected API Key ID. It never stores or receives the plaintext API Key.
- Canvas configuration UI displays the Sub2API user's avatar and username/email from the SSO session, not a generic site label.
- Canvas server-side proxy resolves the selected Key for each request using an internal service token, then forwards to Sub2API `/v1/...`.
- Canvas provides a return-to-main-site action and a logout action. Logout clears the Canvas session first, then sends the browser to Sub2API `/canvas/logout` so the main-site token can be cleared too.

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
- `SUB2API_INTERNAL_BASE_URL=http://sub2api-ca:8080`
- `SUB2API_PUBLIC_BASE_URL=https://api.smallice.xyz` for server-to-server internal API fallback and public API references
- `SUB2API_WEB_BASE_URL=https://smallice.xyz` for browser login, return-to-main-site, and logout redirects. If absent, Canvas falls back to `SUB2API_PUBLIC_BASE_URL`.
- `SUB2API_CANVAS_INTERNAL_TOKEN=<same shared random secret>`
- `CANVAS_SESSION_SECRET=<random session signing secret>`

The Canvas frontend uses the fixed same-origin base URL `/api/sub2api`. It hides manual Base URL and plaintext API Key input.
`CANVAS_PUBLIC_BASE_URL` is used after SSO callback success or failure, so it must be the browser-visible Canvas origin rather than `0.0.0.0` or a container address.
Canvas does not use `CANVAS_SITE_NAME` for user-facing account display; it reads avatar and username/email from the SSO session.

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

The SSO exchange response includes `id`, `email`, `username`, `avatar_url`, and `role`. Canvas stores those non-secret fields in its signed HttpOnly session cookie.

## Image Key Eligibility

A Key is selectable only when all conditions are true:

- It belongs to the current user.
- It is active.
- It is not expired.
- Its configured quota is not exhausted.
- Its group allows image generation. Ungrouped keys keep existing Sub2API behavior.
- The relevant OpenAI account pool has at least one schedulable account with `supports_image_generation=true`.

The public Canvas key-list response includes only selection fields: `id`, `name`, `masked_key`, `group_name`, `expires_at`, `quota`, `quota_used`, and `image_eligible`.

It must not include email, username, user ID, plaintext API Key, account ID, or group ID. The Canvas UI should not display `masked_key` in the selector; it is retained only as an API compatibility field.

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
- `frontend/src/views/user/CanvasLaunchView.vue`
- `frontend/src/views/auth/CanvasLogoutView.vue`
- `frontend/src/router/index.ts`
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
- `/Users/kevin/Documents/API/infinite-canvas/web/src/components/layout/user-status-actions.tsx`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/components/layout/sub2api-theme-handoff.tsx`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/proxy.ts`
- `/Users/kevin/Documents/API/infinite-canvas/web/src/services/api/image.ts`

## Risks

- Duplicate/multi-replica SSO ticket state: current in-memory ticket store is safe only for one Sub2API process.
- Plaintext Key leakage: never return the resolved API Key to the browser; only the Canvas server-side proxy may receive it.
- Cross-user Key use: every resolve call must check `user_id + api_key_id` ownership and eligibility.
- Eligibility drift: group image permission and account `supports_image_generation` must remain aligned with the gateway's image-generation enforcement.
- Login redirect drift: direct Canvas visits must use the public main-site web origin, not the API origin, otherwise users see the wrong domain and may not return to Canvas after login.
- Runtime env drift: `SUB2API_WEB_BASE_URL` is served by the Canvas session API because Docker runtime env must control the link even when `NEXT_PUBLIC_*` values were fixed at image build time.
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
- Open `https://canvas.smallice.xyz/` while logged out and confirm it redirects to `https://smallice.xyz/login?redirect=/canvas/launch`, then returns to Canvas after login.
- Reuse the same ticket and confirm exchange fails.
- Confirm Canvas shows the user's avatar and username/email, not `Sub2API` or `CANVAS_SITE_NAME`.
- Confirm the Canvas Key selector does not show the masked `sk-...` value.
- Confirm return-to-main-site goes to `SUB2API_WEB_BASE_URL`, and logout clears the Canvas session and lands on Sub2API login.
- Confirm light/dark mode from Sub2API is reflected after opening Canvas.
- Confirm only image-eligible Keys appear.
- Confirm browser storage contains selected Key ID, not plaintext API Key.
- Confirm `/v1/models`, Responses, image generation, and image edits go through Sub2API and produce existing usage records.
