# Plan: notification robustness + WhatsApp channel

Spec: `docs/superpowers/specs/2026-09-06-whatsapp-and-robustness-design.md`

Each task: write/adjust the test first, implement, `go build ./... && go vet ./...`, run the package tests.

1. **models** — add `RouteTypeWhatsAppForm`; make `Notification.ToAPI` nil-safe for status/language; add `SelectMessageBody` with the fallback chain (R2, R4).
2. **events/route_preference.go** — `routeLookup`, `ErrNoRouteMatched`, `channelTypesForContact`, `routeWithChannelPreference`; wire into `out.route` and `in.route` (R5, R10, §5.2). Existing untracked tests must pass.
3. **repository** — `NotificationRepository.GetByExternalID`; migration adding the `external_id` index (R6, §6).
4. **business** — Release updates `released_at` + emits `out.route` (R1); Search status map fix (R2); TemplateSave error return (R3); StatusUpdate resolves by external id (R6).
5. **events/status_save** — persist `external_id`; one-hop fallback child on FAILED + `fallback_channel` (§5.3).
6. **events/out_queue + in_queue** — use `SelectMessageBody`; add `whatsapp_template` extra; publish proto bytes on inbound (R8, §5.4).
7. **pkg/integration/credentials** — shared resolver (connection header → route id → settings) (R7).
8. **africastalking** — use the resolver; typed webhook parsing; delivery report via external id; incoming SMS → Receive (R7, R9).
9. **whatsapp integration** — config, client (+tests), webhook types/signature (+tests), handlers (+tests with in-process Connect server), queue worker, main, Dockerfile, Makefile `APP_DIRS`.
10. **client/templates** — `ChannelWhatsApp`.
11. Full `make vet`, `go test -race ./...`; commit; PR.
