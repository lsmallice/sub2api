# Image Capability Customization

Last updated: 2026-07-08

Current custom branch: `image-capability`

Current upstream base at the time of this document: `0.1.147`

## Purpose

This document records the custom Sub2API image-generation permission and routing changes. Keep it updated whenever the feature changes so future upstream merges can preserve the intended behavior without guessing from scattered code.

The upstream `main` branch remains the primary codebase. This customization is an overlay. During every upstream merge, prefer upstream structure and fixes first, then reapply or adapt this feature only where it is still valid. If a custom behavior no longer works after upstream changes, do not release it as-is. Rework it and run the regression checklist below.

## Core Rules

- Group-level `allow_image_generation` controls whether users in the group are allowed to generate images and how image usage is billed.
- Account-level image gating uses upstream's `Account.SupportsOpenAIImageCapability(...)` semantics. The custom `supports_image_generation` column is legacy compatibility data and must not be used as a scheduling, Canvas, Draw, or UI switch. Do not add new admin controls for this field.
- Text requests must keep the normal OpenAI account scheduling path.
- Image-generation requests must select only accounts accepted by upstream OpenAI image capability checks, currently OpenAI OAuth or API Key accounts.
- For non-streaming OpenAI Images API requests with `response_format=url`, Sub2API must force the upstream request to `b64_json`, persist the returned base64 image under `/images/smallice/`, and return a signed public URL that expires in 1 hour. Requests with `response_format=b64_json` must keep the upstream/client base64 response shape. Streaming image requests keep image payloads in `b64_json` events and do not generate public URLs.
- Generated public image files are cleaned lazily on save/read. Cleanup runs at most once every 15 minutes and deletes files whose modification time is older than the 1-hour public URL TTL.
- Docker-internal tool callers such as Smallice Draw and Infinite Canvas must not leak internal hosts like `backend:6666` in generated image URLs. Configure `gateway.openai_images_public_base_url` or `OPENAI_IMAGES_PUBLIC_BASE_URL` to the browser-reachable API origin, for example `https://api.smallice.xyz`.
- If the group does not allow image generation, return the existing user-facing error: `Image generation is not enabled for this group`.
- If the group allows image generation but no image-capable OpenAI account is available, return `no_image_capable_account`.
- Custom builds display `imgcap-<base-version>` but compare updates against the upstream base semver, not the custom label.

## Data Model

Legacy compatibility column:

```sql
supports_image_generation BOOLEAN NOT NULL DEFAULT false
```

Migration file:

- `backend/migrations/144_add_account_supports_image_generation.sql`

Ent and repository touchpoints:

- `backend/ent/schema/account.go`
- generated Ent files under `backend/ent/`
- `backend/internal/repository/account_repo.go`
- `backend/internal/service/account.go`
- `backend/internal/service/account_service.go`
- `backend/internal/service/admin_service.go`

Merge note:

- If upstream adds a migration with the same number, keep upstream first and renumber this migration to the next available migration number. Also verify any migration registry or embedded migration ordering still sees the renamed file.
- Existing accounts default to `false`, but this field is no longer used for scheduling or user-facing admin configuration. Do not require production accounts to be explicitly marked capable after upstream image capability support is present, and do not re-enable a manual account-level switch while upstream capability detection exists.

## Request Classification

The shared classifier lives in:

- `backend/internal/service/image_generation_intent.go`

Primary APIs:

- `ClassifyRequestCapability(endpoint, requestedModel, body)`
- `ClassifyRequestCapabilityMap(endpoint, requestedModel, reqBody)`
- `IsImageGenerationIntent(...)`
- `IsImageGenerationIntentMap(...)`

Classification result:

```go
type RequestCapabilityClassification struct {
    IsImageGeneration     bool
    ImageGenerationSource string
}
```

Current sources:

- `images_api`
- `responses_tool`
- `chat_image_model`
- `chat_image_modalities`
- `image_model`
- `none`

Current recognition rules:

- `/v1/images/generations`, `/images/generations`: image generation.
- `/v1/images/edits`, `/images/edits`: image generation or edit.
- Dedicated image endpoints accept custom/non-`gpt-image-*` model IDs; the endpoint itself establishes image intent, and model mapping/upstream validation handle provider-specific names.
- `/v1/responses`: image generation when `tool_choice` selects `image_generation`.
- `/v1/responses`: a plain top-level `tools` declaration for `{ "type": "image_generation" }` or `{ "type": "namespace", "name": "image_gen" }` is not image intent by itself. Text requests that only declare these tools strip image-generation declarations before upstream forwarding, which avoids blocking normal text traffic just because an earlier client context exposed image tools.
- `/v1/chat/completions`: image generation when `model` starts with an OpenAI image model prefix such as `gpt-image-`.
- `/v1/chat/completions`: image generation when `modalities` contains `image`.
- Any endpoint with an image-generation model in `requestedModel` or request body `model` is classified as `image_model`.

Merge note:

- Keep classifier logic centralized. Do not duplicate endpoint-specific image detection in handlers, billing, or scheduler code.
- If upstream introduces a new OpenAI image endpoint, add it here first, then route it through the same group permission and account capability checks.

## Permission Flow

Shared helpers:

- `GroupAllowsImageGeneration(group *Group) bool`
- `ImageGenerationPermissionMessage() string`
- `NoImageCapableAccountMessage() string`
- `ErrNoImageCapableAccount`

Rules:

- `nil` group keeps legacy behavior and is allowed.
- Non-nil group must have `AllowImageGeneration=true`.
- Group permission is checked before account selection for image traffic.
- Account capability is checked during account selection.

Expected errors:

- Group disabled: HTTP 403, code `permission_error`, message `Image generation is not enabled for this group`.
- No capable account: HTTP 403, code `no_image_capable_account`, message `No image-capable OpenAI account is configured for this group`.

## Scheduling Flow

Main scheduler touchpoint:

- `backend/internal/service/openai_account_scheduler.go`

Important APIs:

- `SelectAccountWithSchedulerForImageIntent(...)`
- `SelectAccountWithSchedulerForImages(...)`
- `selectAccountWithScheduler(... requiredImageCapability ...)`
- `accountSupportsOpenAICapabilities(...)`
- `Account.SupportsOpenAIImageCapability(...)`

`SupportsOpenAIImageCapability` currently returns true only when:

- the account is OpenAI,
- the account type is OAuth or API key,
- the required image capability is `OpenAIImagesCapabilityBasic` or `OpenAIImagesCapabilityNative`.

For non-image capability checks, it returns true by default so normal text scheduling is unaffected.

Dedicated image endpoints call `SelectAccountWithSchedulerForImages`. Responses and Chat Completions image intents call `SelectAccountWithSchedulerForImageIntent`. Both paths pass a non-empty `RequiredImageCapability`, which filters accounts by `SupportsOpenAIImageCapability`.

Merge note:

- If upstream changes scheduler request structs, preserve the equivalent of `RequiredImageCapability`.
- If upstream changes load-aware selection, keep the filtering semantic: image requests cannot fall through to accounts that fail `SupportsOpenAIImageCapability`.
- If upstream adds a new account type that can handle OpenAI image generation, update `SupportsOpenAIImageCapability` and tests deliberately.

## Handler Integration

OpenAI HTTP handlers:

- `backend/internal/handler/openai_images.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_image_generation_logging.go`

Expected handler behavior:

- Parse and validate request as upstream normally does.
- Classify request capability.
- For image intents, log `request_capability=image_generation` and `image_generation_source`.
- Check group image permission before scheduling.
- Select with image-aware scheduler path.
- Log `selected_account_supports_openai_image_generation=true|false` for image intents.
- Forward the original request body except for upstream-approved model mapping, existing service transforms, and the non-streaming Images `response_format=url` transform that forces upstream `b64_json`.

Responses WebSocket and forwarder touchpoints:

- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/openai_ws_forwarder.go`

Merge note:

- If upstream rewrites Responses streaming or WebSocket handling, re-audit both HTTP and WS paths. They must share the same classification and permission behavior.

## Admin API And UI

Backend admin surfaces:

- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_data.go`
- `backend/internal/handler/admin/account_codex_import.go`
- `backend/internal/handler/admin/openai_oauth_handler.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`

Frontend admin surfaces:

- `frontend/src/types/index.ts`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/components/admin/account/AccountTableFilters.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Expected behavior:

- Account responses, backups, and import paths must not expose or accept `supports_image_generation`; keep it as a database compatibility column only.
- Create, edit, bulk edit, import/export, and account list UI must not expose `supports_image_generation` as a user-facing switch, badge, or filter.
- Account-level Codex image tool overrides (`codex_image_generation_bridge`, `codex_image_generation_bridge_enabled`, `codex_image_generation_explicit_tool_policy`) are legacy keys and must not affect routing. Account edit saves should remove these keys; official account capability checks plus channel/global bridge settings remain the supported control surface.
- Image-capable Key eligibility for Canvas and Draw must use the same `SupportsOpenAIImageCapability` path as gateway scheduling.
- Admin DTO responses must not expose raw secrets. Keep secret redaction behavior intact.

Merge note:

- If upstream changes account forms or table filters, keep the custom account-level image switch removed unless upstream introduces a first-party user-facing control with different product semantics.
- Do not reintroduce `supports_image_generation` into scheduling or tool-site eligibility during upstream merges.
- Treat `supports_image_generation` as database compatibility data only; official OpenAI capability checks, channel/global bridge settings, and group `allow_image_generation` remain the supported admin controls.

## Custom Version Display

This branch also carries a production-safety version customization.

Backend touchpoints:

- `backend/cmd/server/main.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/service/update_service.go`
- `backend/internal/handler/admin/system_handler.go`

Frontend touchpoints:

- `frontend/src/api/admin/system.ts`
- `frontend/src/stores/app.ts`
- `frontend/src/components/common/VersionBadge.vue`
- `frontend/src/stores/__tests__/app.spec.ts`

Build variables:

- `Version=<upstream base semver>`, for example `0.1.137`.
- `BuildLabel=imgcap-<base-version>`, for example `imgcap-0.1.137`.
- `BuildType=custom`.
- `Commit=<shortsha>`.

Expected behavior:

- Admin and public UI display `imgcap-<base-version>`.
- Update comparison uses `current_version` or `base_version`, not `display_version`.
- `has_update=true` only when GitHub latest is greater than the upstream base semver.
- `custom` builds do not show the one-click upstream update button.

Merge note:

- If upstream changes update-check logic, preserve the split between base semver and display label.
- Never set `Version` to a Docker tag such as `imgcap-0.1.137-7965a1ff`.

## Release Naming

Production image tags use:

```text
sub2api:imgcap-<base-version>-<shortsha>
```

Rollback tags use:

```text
sub2api:rollback-pre-imgcap-<timestamp>
```

Production should not run `weishaw/sub2api:latest` for this customized deployment.

## Merge Checklist

Use this checklist whenever upstream `main` is merged into `image-capability`.

1. Read this document before resolving conflicts.
2. Treat official upstream `main` as the base truth for unrelated behavior, security fixes, refactors, and bug fixes.
3. Reapply this customization only where needed, using the new upstream structure.
4. Search for all custom anchors:

```bash
rg -n "SupportsOpenAIImageCapability|ClassifyRequestCapability|RequestCapabilityClassification|ErrNoImageCapableAccount|NoImageCapableAccountMessage|selected_account_supports_openai_image_generation" backend frontend
rg -n "BuildLabel|BuildType|display_version|base_version|build_type|imgcap" backend frontend
```

5. Verify the legacy migration still exists or has been safely renumbered.
6. Verify handlers call the shared classifier instead of local image-detection snippets.
7. Verify image requests use an image-aware scheduler path.
8. Verify normal text requests still use normal OpenAI scheduling.
9. Verify admin create, edit, list, filter, and bulk edit do not expose the legacy field.
10. Verify custom version display and update checks still use base semver.
11. Update this document if any behavior, file path, or test command changes.

If any item fails because upstream changed the architecture, pause the release and rework the feature. Do not ship with partial image routing or broken version/update behavior.

## Regression Test Plan

Backend tests:

```bash
go test ./internal/service -run 'TestUpdateService|TestOpenAIGatewayService_SelectAccountWithScheduler|TestAccountSupportsOpenAI|TestClassifyRequestCapability|TestIsImageGeneration'
go test ./internal/handler -run 'Test'
go test ./...
```

Frontend build:

```bash
cd frontend
corepack pnpm@9 run build
```

Manual checks:

- Old accounts migrate with `supports_image_generation=false`, but the value does not block official OpenAI image-capable account selection.
- Creating, editing, listing, filtering, and bulk editing accounts does not expose a custom account-level image switch.
- Editing an OpenAI account clears legacy account-level Codex image tool override keys.
- `/v1/images/generations` selects only an upstream image-capable OpenAI account.
- `/v1/images/generations` accepts custom image model IDs such as provider aliases instead of rejecting everything outside `gpt-image-*`.
- Non-streaming `/v1/images/generations` with `response_format=url` sends upstream `response_format=b64_json`, returns a signed `/images/smallice/...` URL, and removes `b64_json` from the client response.
- Streaming `/v1/images/generations` with `response_format=url` returns image SSE events with `b64_json` and does not produce public image URLs.
- `/v1/images/edits` selects only an upstream image-capable OpenAI account.
- `/v1/responses` with only `tools: [{ "type": "image_generation" }]` stays on the normal text path and strips the image tool declaration before forwarding.
- `/v1/responses` with `tool_choice: {"type":"image_generation"}` selects only an upstream image-capable OpenAI account.
- `/v1/responses` text-only requests do not require an image-capable account.
- `/v1/chat/completions` with `gpt-image-*` or `modalities:["image"]` selects only an upstream image-capable OpenAI account.
- Group `allow_image_generation=false` rejects image requests even when image-capable accounts exist.
- Group `allow_image_generation=true` with no flagged account returns `no_image_capable_account`.
- Text chat in Open WebUI still works.
- Smallice Draw image generation and edits still work.
- Admin version badge displays `imgcap-<base-version>`.
- GitHub latest equal to base version does not show an update.
- GitHub latest greater than base version shows a manual custom-build update hint, not the one-click update button.

## Production Validation

After deployment, verify:

```bash
curl -i --max-time 10 https://api.smallice.chat/health
curl -i --max-time 10 https://api.smallice.xyz/health
curl -s --max-time 10 https://api.smallice.chat/api/v1/settings/public
```

Database checks should confirm:

- `accounts.supports_image_generation` exists only as a legacy compatibility column.
- Intended OpenAI OAuth/API Key accounts are schedulable and pass `SupportsOpenAIImageCapability`.

Do not print account secrets or `.env` values while validating production.
