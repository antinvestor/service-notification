# Notification robustness and WhatsApp channel — design

**Status:** implemented on branch `feat/whatsapp-channel` (autonomous session, assumptions flagged in §8)
**Date:** 2026-09-06
**Author:** Peter Bwire (with Claude Code)
**Scope:** `apps/default` (core pipeline), `apps/integrations/africastalking`, new `apps/integrations/whatsapp`, `client/templates`

## 1. Goal

1. Fix the defects in the existing notification pipeline that currently break or
   silently degrade core flows (release, search, template rendering, delivery
   reports, credentials resolution).
2. Add WhatsApp as a first-class channel: outbound sends (text and pre-approved
   template messages) and inbound receipt (user messages and delivery/read
   statuses) via the Meta WhatsApp Business Cloud API, with routing that prefers
   WhatsApp for phone contacts and falls back to SMS when WhatsApp is not
   available for the partition or for the specific recipient.

## 2. Non-goals

- Media sending/receiving beyond recording that a media message arrived
  (captions and a `media_id` are captured; downloading media is out of scope).
- Interactive messages (buttons, lists, flows).
- WhatsApp template registration/management with Meta. Templates are approved
  in Meta Business Manager; this service only references them by name.
- Changing the public proto contract. Everything rides on existing fields
  (`type`, `extras`, `route_id`, template `extra`).
- A UI for routes. Routes stay database rows as today.

## 3. Current pipeline (for reference)

```
Send RPC ─► QueueOut ─► notification.save ─► notification.out.route ─► notification.out.queue ─► route queue (proto bytes + headers)
                                                                                                        │
                                                                                     integration worker ┘ ─► provider API
                                                                                     integration webhook ─► StatusUpdate RPC / Receive RPC
Receive RPC ─► QueueIn ─► notification.save ─► notification.in.route ─► notification.in.queue ─► route queue (consumer app)
```

## 4. Robustness defects and fixes (core)

| # | Defect | Effect today | Fix |
|---|--------|--------------|-----|
| R1 | `Release` re-emits `notification.save` for an existing row; the save handler treats the duplicate-key error as "already saved" and returns without emitting `out.route`. | Released notifications are **never dispatched**. The RPC still reports QUEUED. | `Release` updates `released_at` through the repository and emits `notification.out.route` directly. The save handler keeps its idempotent duplicate guard. |
| R2 | `convertNotificationsToAPI` keys the status map by `notification.ID` instead of `StatusID`; `ToAPI` dereferences a possibly nil language. | Search returns notifications with no status; a notification whose language row is missing panics the stream. | Key by `StatusID`; `ToAPI` tolerates nil status and nil language. |
| R3 | `TemplateSave` returns `nil, err` (err is nil) when loading template data fails. | Caller gets an empty success. | Return the real error. |
| R4 | Outbound body selection only tries `message[type]` then `message["sms"]`. Seeded templates use type `text`. | Templated messages for `whatsapp`/`text` templates go out empty. | Fallback chain: exact type → `text` → `default` → `sms` → first available. Extracted to `selectMessageBody` (unit tested). |
| R5 | Route failures are detected by `strings.Contains(err.Error(), ...)`. | Fragile. | Sentinel `ErrNoRouteMatched` and `errors.Is`. |
| R6 | `StatusUpdate` requires the notification id; delivery reports from providers only carry the provider's message id. `ExternalID` is never persisted on the notification. | Africa's Talking delivery reports always fail (`Id: ""`). WhatsApp status webhooks would too. | Status save persists `external_id` onto the notification. `StatusUpdate` accepts an external id when the id is empty and resolves it via `GetByExternalID`. |
| R7 | Out-queue metadata only carries tenant/partition/route ids. The Africa's Talking client requires `X-API_CONNECTION_CREDENTIALS` or explicit key headers. | SMS sends fail with "no api key exists" unless something outside this repo injects headers. | Integrations resolve credentials from the settings service using the connection header when present, otherwise the route id. (Core is unchanged; fix lives in `pkg/integration/credentials`.) |
| R8 | Inbound queue publishes the internal `models.Notification` struct; outbound publishes proto bytes with headers. | Consumers of inbound routes depend on an internal Go struct's JSON shape. | Inbound queue publishes proto bytes with the same headers as outbound. Flagged as a wire change in §8. |
| R9 | Africa's Talking webhook does unchecked type assertions on JSON (`payload["id"].(string)`, `networkCode.(int)` where JSON numbers decode as `float64`). | Panics on malformed or numeric fields. | Typed, tolerant extraction helpers. Incoming SMS handler implemented (calls `Receive`). |
| R10 | Out-route overwrites the client-supplied `type` purely from contact type. | Clients cannot request a specific channel. | See §5.2: explicit compatible type wins, otherwise contact preference chain. |

Fallback on delivery failure (§5.3) is also a robustness feature.

## 5. WhatsApp channel

### 5.1 Route type and channel constants

`models.RouteTypeWhatsAppForm = "whatsapp"`. `client/templates` gains
`ChannelWhatsApp = "whatsapp"`. A partition enables WhatsApp by inserting a
route row with `route_type='whatsapp'`, `mode='tx'` (or `trx`) and a queue URI
consumed by the WhatsApp integration; inbound consumers register an `rx` route.

### 5.2 Outbound routing with channel preference

In `notification.out.route`:

```
channelTypesForContact(MSISDN) = [whatsapp, sms]
channelTypesForContact(EMAIL)  = [email]
channelTypesForContact(other)  = [any]
```

`routeWithChannelPreference(ctx, routeLookup, mode, n, channelTypes)`:

1. If `n.RouteID` is set, load that route; channel = `n.NotificationType`
   (or the route's type when empty). No fallback.
2. Else if `n.NotificationType` is one of `channelTypes`, try only that
   channel (explicit request wins).
3. Else try each channel in order; the first with a matching route wins. The
   returned `fallback` flag is true when a channel after the first was used, and
   is recorded in the status extra (`channel_fallback: true`) for observability.
4. No route on any channel → `ErrNoRouteMatched` → FAILED status (as today).

`routeLookup` is the narrow interface (`GetByID`, `GetByModeTypeAndPartitionID`)
already assumed by the existing test files.

### 5.3 Fallback on delivery failure

When an integration determines a message cannot be delivered on its channel but
could be on another (WhatsApp: recipient has no WhatsApp account, or the
business-initiated window requires a template that was not supplied), it reports
`STATUS_FAILED` with extras `fallback_channel: "sms"`.

`notificationStatus.save` handles this: for an outbound notification whose new
status is FAILED with a non-empty `fallback_channel`, and whose `ParentID` is
empty (one hop only), it creates a child notification:

- copy of the parent (recipient, sender, template, payload, language, message,
  priority, partition/tenant/access),
- `ParentID = parent.ID`, `NotificationType = fallback_channel`, `RouteID = ""`,
  `ReleasedAt = now`,

and emits `notification.save`, which flows through routing normally. Rule 2 in
§5.2 makes the child go only to the fallback channel. The parent stays FAILED
with `fallback_notification_id` in its status extra.

### 5.4 Outbound payload to the integration

Unchanged proto. `Extras` (already a `type → body` map) gains:

- `whatsapp_template` — JSON string of the template's `extra["whatsapp"]`
  object when the notification was rendered from a template that defines one:
  `{"name": "otp_code", "language": "en_US", "params": ["code", "expiryDate"]}`.
  `params` lists payload keys in the order of the Meta template's body
  placeholders. The rendered `payload` values are sent as body parameters.

Templates without a WhatsApp definition send free-form text (works inside the
24h customer-service window; outside it Meta returns error 131047, which triggers
§5.3 fallback to SMS).

### 5.5 Integration app `apps/integrations/whatsapp`

Mirrors the Africa's Talking layout.

```
apps/integrations/whatsapp/
  cmd/main.go                    frame service: HTTP webhook + queue subscriber + status-update event
  config/config.go               queue name/URI, Graph API base URL + version, verify token, app secret, settings ids
  service/client/client.go       Cloud API client: SendText, SendTemplate, error classification
  service/client/webhook.go      typed webhook payload structs + signature verification
  service/handlers/webhook.go    GET verify (hub.challenge), POST events → Receive / StatusUpdate
  service/queue/messages_to_send.go  queue worker: proto → send → status update event
  Dockerfile
```

**Credentials** per connection come from the settings service as JSON:
`{"access_token": "...", "phone_number_id": "...", "app_secret": "...", "verify_token": "..."}`
resolved via `pkg/integration/credentials` (connection header → route id).
`app_secret`/`verify_token` may also be set globally by env for single-tenant
deployments.

**Send**: `POST {base}/{version}/{phone_number_id}/messages` with
`messaging_product: whatsapp`, `to: <E.164 without '+'>`, `type: text|template`.
Response `messages[0].id` (wamid) becomes `ExternalId`; status reported as
QUEUED (delivery confirmed later by webhook).

**Error classification** (Meta error `code`):

| code | meaning | action |
|------|---------|--------|
| 131026 | recipient not a WhatsApp user / cannot receive | FAILED, `fallback_channel=sms` |
| 131047 | re-engagement window expired (needs template) | FAILED, `fallback_channel=sms` |
| 131030, 131021, 100, 131008, 131009 | bad recipient/params | FAILED (no fallback) |
| 130429, 131056, 131048, 80007 | throttling / pair rate limit / spam rate | retriable (STATUS_UNKNOWN, return error so the queue redelivers) |
| 0, 1, 2, 131000, 131016, 5xx HTTP | transient service error | retriable |
| 190, 10, 200-299 | auth / permission | FAILED, non-retriable, logged at error |

**Webhook**: `GET /receive/notification/{routeID}` answers the verification
challenge when `hub.verify_token` matches. `POST` validates
`X-Hub-Signature-256` (HMAC-SHA256 of the raw body with the app secret) when a
secret is configured, then for every `entry[].changes[].value`:

- `messages[]` → `Receive` with `type=whatsapp`, `route_id=routeID`,
  `source.detail=from` (E.164 with '+'), `source.profile_name=contacts[].profile.name`,
  `recipient.detail=metadata.display_phone_number`, `data=text.body`
  (or caption), `extras={wamid, timestamp, message_type, media_id, context_wamid}`.
  Idempotency: the notification `id` is derived deterministically from the wamid
  so redelivered webhooks dedupe on the existing duplicate-key guard.
- `statuses[]` → `StatusUpdate` with `external_id=wamid`:
  `sent → IN_PROCESS`, `delivered → SUCCESSFUL`, `read → SUCCESSFUL (extra read=true)`,
  `failed → FAILED (extra errors)`.

Always respond 200 once the payload is parsed, even if a single item fails
(logged), so Meta does not retry-storm; a signature or parse failure returns
401/400.

## 6. Data model changes

- `notifications.external_id` is now written (column exists). Add an index
  migration `20260906_notification_external_id.sql` on `(external_id)`.
- No new tables.

## 7. Testing

- Core unit tests: `channelTypesForContact`, `routeWithChannelPreference`
  (files already present), `selectMessageBody`, fallback child construction,
  webhook helpers.
- Core integration tests (existing Postgres-backed suites): Release now leaves
  `released_at` set and emits out-route; Search returns statuses; status update
  by external id.
- WhatsApp client tests against `httptest` servers: text and template request
  shapes, each error-classification row.
- Webhook handler tests: verification handshake, signature accept/reject,
  inbound message → `Receive` call, status → `StatusUpdate` call, using an
  in-process Connect server for `NotificationService` (no mocks of the client).
- `go build ./...`, `go vet ./...`, `go test -race ./...` must pass (pre-push
  hook enforces this).

## 8. Assumptions flagged for the owner

1. **Provider**: Meta WhatsApp Business Cloud API directly (not Twilio/360dialog).
   The client is isolated in `service/client`, so a different provider is a
   client swap.
2. **WhatsApp preferred over SMS** for phone contacts when both routes exist in a
   partition. A partition that wants SMS-only simply has no WhatsApp route.
3. **R8 wire change**: inbound route consumers now receive proto bytes with the
   same headers as outbound. No consumer exists in this repo; external consumers
   (if any) must switch to proto decoding.
4. **One-hop fallback** only (WhatsApp → SMS). Never SMS → WhatsApp.
