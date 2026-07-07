# Smallice Draw Tool Integration

Last updated: 2026-06-29

## Purpose

Expose Smallice Draw as a Sub2API user tool without asking users to maintain an API Base URL or plaintext API Key in the Draw UI.

Sub2API remains the source of login state, API Key ownership, image-generation eligibility, billing, quota, routing, usage logs, and risk controls. Smallice Draw is a separate Docker-deployed tool site that receives a short browser launch handoff from the Sub2API frontend and then proxies image requests through Sub2API.

## User Flow

1. A logged-in user clicks `Smallice Draw` in the Sub2API sidebar.
2. The frontend redirects to `VITE_SMALLICE_DRAW_URL` or the default `https://draw.smallice.xyz`.
3. The current theme is passed as a query parameter.
4. The Sub2API access token is passed in the URL hash as `#token=...`, so it is not sent in the HTTP request line to the Draw host.
5. Draw immediately posts the token to `/tools/draw-api/session/bootstrap`, stores it in a HttpOnly cookie, and removes the token from the visible URL.

## Eligibility

Smallice Draw can only select API Keys that are eligible for image generation.

The Draw proxy first calls Sub2API `GET /api/v1/canvas/image-keys` with the user's token. That endpoint applies the strict image-key policy:

- Key belongs to the current user.
- Key is active, unexpired, and not quota-exhausted.
- The bound group allows image generation.
- The group has at least one schedulable OpenAI account accepted by `SupportsOpenAIImageCapability`.

The Draw proxy then reads the user's regular key list only to resolve the plaintext Key for server-side forwarding, and keeps only IDs returned by the image-key eligibility endpoint. Plaintext Keys are never returned to the browser.

## Code Anchors

- Sub2API frontend launch:
  - `frontend/src/utils/drawLaunch.ts`
  - `frontend/src/views/user/DrawLaunchView.vue`
  - `frontend/src/components/layout/AppSidebar.vue`
  - `frontend/src/router/index.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`
- Draw proxy:
  - `/Users/kevin/Documents/API/smallice-draw/proxy/server.mjs`
  - `/Users/kevin/Documents/API/smallice-draw/src/lib/smalliceSession.ts`
  - `/Users/kevin/Documents/API/smallice-draw/src/components/SettingsModal.tsx`

## Deployment Notes

- Sub2API frontend can override the target Draw site with `VITE_SMALLICE_DRAW_URL`.
- Draw Docker service must set `SUB2API_BASE_URL` to the internal Sub2API URL, such as `http://sub2api-ca:8080`.
- Draw's public reverse proxy must route `/tools/draw-api/*` to the Draw proxy service and all SPA paths to the static frontend.

## Regression

- `corepack pnpm@9 run typecheck` from `frontend/`.
- `corepack pnpm@9 run build` from `frontend/`.
- `npm run build` from `/Users/kevin/Documents/API/smallice-draw`.
- Manual check: sidebar opens Draw, Draw bootstrap removes the token from the URL, user info loads, only image-capable Keys appear, and an image request bills through Sub2API.
