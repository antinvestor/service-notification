###service-notification

![project tests](https://github.com/antinvestor/service-notification/actions/workflows/run_tests.yml/badge.svg) ![image release](https://github.com/antinvestor/service-notification/actions/workflows/release.yml/badge.svg)


A repository for the  notification service being developed
for ant investor

## Layout

| Path | Purpose |
|------|---------|
| `apps/default` | Notification service: Connect RPC API, routing, templating, status tracking |
| `apps/integrations/africastalking` | SMS via Africa's Talking (send queue worker + delivery/inbound webhook) |
| `apps/integrations/emailsmtp` | Email via SMTP |
| `apps/integrations/smpp` | SMS via SMPP |
| `apps/integrations/whatsapp` | WhatsApp via the Meta Cloud API (see its README for setup) |
| `apps/ussd` | USSD menu engine |
| `client/templates` | Template registration contract for consumer services |

## Channels and routing

A notification is routed by the recipient contact type and the routes configured for
the partition (`routes` table: `mode` tx/rx/trx, `route_type` email/sms/whatsapp/any, `uri`):

- phone contacts prefer `whatsapp` and fall back to `sms` when the partition has no WhatsApp route;
- email contacts use `email`;
- an explicit `type` on the notification restricts routing to that channel, and an explicit
  `route_id` bypasses routing entirely.

When an integration cannot deliver on its channel it may report `STATUS_FAILED` with the
extra `fallback_channel`; the service then sends one child notification (`parent_id` set)
over that channel.

Delivery reports from providers resolve notifications through the provider message id
(`external_id`), so `StatusUpdate` accepts an `external_id` without an `id`.

## Development Setup

### Git Hooks

This repository includes a pre-commit hook that automatically runs `make format` before each commit to ensure consistent code formatting.

**Enable the hook:**
```bash
git config core.hooksPath .githooks
```

**What it does:**
- Detects staged `.go` files
- Runs `make format` to apply gofmt/goimports
- If formatting changes any files, the commit is blocked
- You must review and stage the formatted files before committing again

**To disable temporarily:**
```bash
git commit --no-verify
```

