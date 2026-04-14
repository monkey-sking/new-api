# Upstream Account Check-in Design

**Date:** 2026-04-08
**Workstream:** `feature/upstream-account-checkin`
**Status:** approved-in-conversation

## Goal

Replace the current channel-bound manual upstream check-in flow with a reference-style upstream operations model:

- upstream site management
- upstream account/session management
- automatic access token acquisition
- automatic platform user ID discovery
- account-driven check-in
- check-in history

## Problem

The current implementation stores upstream check-in inputs directly on `channel.settings` and requires manual `access token` and `user id` entry. That is materially different from the reference project, which signs into upstream panel accounts, discovers the usable session/access token, discovers the platform user ID, and runs check-ins against accounts with persistent history.

## Scope

### In Scope

- Add independent upstream site, upstream account, and upstream check-in log models.
- Keep channel auto-detect/preset helpers, but stop relying on channel JSON fields for the primary check-in flow.
- Add backend APIs for:
  - listing and editing upstream sites
  - creating/updating upstream accounts
  - logging into upstream panel accounts to auto-fetch session/access token
  - importing/verifying upstream access tokens
  - auto-discovering platform user ID for new-api-family panels
  - running single-account and scheduled account check-ins
  - listing check-in logs
- Add an upstream operations page organized around `sites -> accounts -> logs`.

### Out of Scope

- Full parity with the reference project's OAuth provider stack.
- Full replacement of existing channel editing and relay logic.
- Automatic channel creation from upstream sites in this round.
- Encrypted password vaulting or secure secret rotation beyond current local-admin assumptions.

## Architecture

### Data Model

Three new tables are introduced:

1. `upstream_sites`
   Stores the upstream panel/base endpoint and platform metadata.

2. `upstream_accounts`
   Stores the account/session state used to sign into upstream panels and perform check-in.

3. `upstream_checkin_logs`
   Stores every check-in result for timeline/history and operator debugging.

Channels remain the relay/control plane. Upstream accounts become the operator-facing control plane for site sessions and check-in behavior.

### Runtime Flow

1. Operator creates or edits an upstream site.
2. Operator adds an upstream account under that site.
3. Account can be configured in two ways:
   - username/password login
   - manual access token import
4. Backend verifies the session and enriches account state:
   - detect usable session/access token
   - discover platform user ID when needed
   - fetch preferred API token when upstream supports it
5. Manual or scheduled check-in runs against the upstream account.
6. Latest account runtime fields are updated and a history record is appended to `upstream_checkin_logs`.

## Reference Alignment

This implementation intentionally copies the reference project's important behavior, not just its labels:

- check-in is account-driven, not channel-driven
- account login auto-fetches session/access token
- new-api-family check-in auto-discovers user ID when needed
- check-in produces persistent history
- upstream operations live behind an independent operator entry, not a hidden channel form section

## Backend Design

### Site Model

`upstream_sites` fields:

- `id`
- `channel_id` optional back-reference for operator convenience
- `name`
- `base_url`
- `platform`
- `preset`
- `proxy`
- `status`
- `extra_config`
- `created_time`
- `updated_time`

### Account Model

`upstream_accounts` fields:

- `id`
- `site_id`
- `name`
- `username`
- `password`
- `access_token`
- `api_token`
- `platform_user_id`
- `checkin_enabled`
- `checkin_interval_hours`
- `last_checkin_at`
- `last_checkin_status`
- `last_checkin_message`
- `last_checkin_reward`
- `status`
- `extra_config`
- `created_time`
- `updated_time`

### Check-in Log Model

`upstream_checkin_logs` fields:

- `id`
- `account_id`
- `status`
- `trigger`
- `message`
- `reward`
- `created_time`

### Platform Support

This round focuses on the panels the current user asked about and the reference path covers well:

- `new-api`
- `one-api`
- `one-hub`
- `veloera`
- `anyrouter`

`done-hub`, `sub2api`, `cliproxyapi`, and official providers remain detectable but do not promise login-driven check-in parity in this round.

## Frontend Design

`/console/channel/upstream` becomes a dedicated operator page with three sections:

1. Sites
2. Accounts
3. Check-in logs

The page should expose:

- add/edit site
- add/edit account
- login/import token
- manual single-account check-in
- account status and last result
- recent history list

The previous channel-bound check-in widgets become secondary compatibility UI and should no longer be the primary path.

## Error Handling

- Unsupported platforms return explicit skip/unsupported status.
- Failed login does not create fake success state.
- Failed user ID discovery is surfaced as account verification failure.
- Check-in failure always appends a history record.
- Scheduled execution skips accounts not due yet.

## Validation

Minimum validation for this workstream:

- targeted Go tests for new models/services/controllers
- frontend build
- focused operator-page lint/static checks
- manual source review for migration and scheduler wiring

## Risks

- This repo already has existing dirty changes; implementation must avoid reverting unrelated work.
- Stored upstream passwords are only acceptable under the current single-admin local-use assumption.
- Some upstream panel variants may require follow-up compatibility fixes even after the reference-aligned structure lands.
