# Custom Features

Last updated: 2026-06-07

This directory records all Sub2API custom features maintained on top of official upstream `main`.

## Rules

- Official upstream `main` is the primary codebase.
- Custom features are overlays and must stay small, explicit, and documented.
- Every new custom feature must add a matching document in this directory.
- Every meaningful behavior, schema, API, UI, release, rollback, or regression-plan change must update the matching document in the same commit.
- When a custom feature is added or materially changed, also update the Codex skill registry:
  - `/Users/kevin/.codex/skills/sub2api-release/references/custom_features.md`
- If a feature changes release, rollback, build, migration, permission, or upstream-merge workflow, also update the relevant skill files:
  - `/Users/kevin/.codex/skills/sub2api-release/SKILL.md`
  - `/Users/kevin/.codex/skills/sub2api-release/references/release_runbook.md`
  - `/Users/kevin/.codex/skills/sub2api-release/references/feature_documentation.md`

## Current Features

- [Image Capability](image-capability.md): routes OpenAI image-generation requests only to accounts marked `supports_image_generation=true`, while group `allow_image_generation` remains the permission and billing source.
- [Subscription Quota Refresh](subscription-quota-refresh.md): lets exhausted daily, weekly, or monthly subscription quota windows refresh early by deducting validity, with idempotent user/admin APIs and an audit trail.
- [Token Leaderboard](leaderboard.md): provides a voluntary, privacy-preserving Token leaderboard with daily, weekly, monthly, all-time Top 10, each opted-in user's own rank, daily aggregate reads, materialized honors, and a manual admin snapshot backfill entry.

## Custom Release And Deployment Workflows

- [imgcap Systemd Installer](imgcap-systemd-installer.md): documents the custom binary release assets and one-click systemd installer for servers that already have PostgreSQL and Redis, such as 1Panel-managed hosts.

## Upstream Merge Rule

During every upstream merge:

1. Read this directory and the skill registry first.
2. Preserve official upstream fixes and structure for unrelated code.
3. Reapply custom behavior as a minimal overlay.
4. If a custom feature no longer works, rework it and run its documented regression plan before release.
5. Update the feature document and skill registry when the final merged behavior changes.
