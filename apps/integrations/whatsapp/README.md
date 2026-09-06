# WhatsApp integration

Sends and receives WhatsApp messages through the Meta WhatsApp Business Cloud API.

## How a message flows

```
Send RPC ──► out.route (MSISDN → whatsapp, else sms) ──► out.queue ──► route queue
                                                                            │
                                            this service (queue worker) ◄───┘
                                            POST /{phone_number_id}/messages ──► Meta
Meta webhook ──► POST /receive/notification/{routeID}
                   ├─ statuses → StatusUpdate (by external id; failed + 131026/131047 → SMS fallback)
                   └─ messages → Receive (type=whatsapp, route_id=routeID) → in.route → consumer queue
```

## Enabling WhatsApp for a partition

1. **Outbound route** (`mode=tx`, `route_type=whatsapp`) whose `uri` is the queue this
   service subscribes to (`QUEUE_NOTIFICATION_WHATSAPP_DEQUEUE_URI`):

   ```sql
   INSERT INTO routes (id, tenant_id, partition_id, name, mode, route_type, uri, created_at, modified_at)
   VALUES ('<route-id>', '<tenant>', '<partition>', 'WhatsApp send', 'tx', 'whatsapp',
           'nats://core-queue-headless.queue-system.svc.cluster.local:4222?subject=svc.notification.integration.whatsapp.send.queue&jetstream=true',
           now(), now());
   ```

   Phone contacts are routed to this route in preference to SMS. Partitions without a
   WhatsApp route keep using SMS. A client can force a channel by setting `type`.

2. **Credentials** in the settings service, keyed by the route id (or by the
   `X-API_CONNECTION_CREDENTIALS` header if your route publisher sets one), under
   object `SETTINGS_INTEGRATION_NAME` / object id `SETTINGS_INTEGRATION_ID`:

   ```json
   {"access_token": "EAAG...", "phone_number_id": "1055...", "app_secret": "...", "verify_token": "..."}
   ```

   `app_secret` and `verify_token` can instead be set service-wide via
   `WHATSAPP_APP_SECRET` / `WHATSAPP_VERIFY_TOKEN`.

3. **Webhook** in Meta App Dashboard → WhatsApp → Configuration:
   callback URL `https://<host>/receive/notification/<rx-route-id>`, subscribed to the
   `messages` field. The `{routeID}` is the **inbound** route (`mode=rx`,
   `route_type=whatsapp`) whose queue your consumer application reads. Statuses arrive
   on the same webhook and resolve by the message id regardless of the route.

## Templates

Free-form text works inside the 24-hour customer-service window. Business-initiated
messages need a Meta-approved template. Register it on the notification template:

```go
templates.Template{
    Name:     "template.profile.contact.verification",
    Language: "en",
    Bodies:   map[string]string{templates.ChannelSMS: "Code {{.code}}", templates.ChannelWhatsApp: "Code {{.code}}"},
    Extra: map[string]any{templates.ExtraWhatsAppKey: map[string]any{
        "name": "otp_code", "language": "en_US", "params": []any{"code", "expiryDate"},
    }},
}
```

`params` (and optional `header_params`) list payload keys in the order of the Meta
template placeholders. When the extra is present the worker sends a template message;
otherwise it sends the rendered `whatsapp` (or `text`/`default`/`sms`) body as text.

## Failure handling

| Situation | Result |
|-----------|--------|
| Rate limits, 5xx, transient errors | status `UNKNOWN`, message left on the queue for redelivery |
| Recipient not on WhatsApp (131026) or re-engagement required (131047) | `FAILED` with `fallback_channel=sms`; the notification service sends a child notification over SMS |
| Bad parameters, unknown template, auth errors | `FAILED`, no fallback; auth errors drop cached credentials and log at error level |
| Webhook signature mismatch | 401, nothing processed |
| One bad item in a webhook batch | logged, rest processed, 200 returned (Meta does not retry) |

Inbound message ids are derived from the WhatsApp message id, so Meta redeliveries are
deduplicated by the notification service.
