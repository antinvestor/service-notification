# USSD

A generic USSD menu engine. A service (the "owner") registers a menu tree and
its gateway settings, pushes the data its menus show each caller, and receives
completed sessions as inbound notifications. Nothing in the engine is specific
to any one service.

## Callers

On a new session the engine looks the dialling number up in the profile
service. The caller's `profile_id` and `contact_id` are put in the session
data as `user_id` and `contact_id`. A number no profile holds gets the menus
that need no data, and its completed sessions are not handed on.

## Gateway

`POST /ussd/gateway/{serviceID}[/{protocol}]` takes form, JSON or XML. The
service's `auth_key` setting must match `authKey` (form field, query
parameter or `X-Auth-Key` header), and `whitelist_ip` limits the source
addresses.

For Africa's Talking, configure the service with:

| setting | value |
|---|---|
| `input_separator` | `*` (AT resends the whole path, `1*2*3`) |
| `data_begin_template`, `data_continue_template` | `CON %s` |
| `data_end_template` | `END %s` |
| `auth_key` | a secret, put in the callback URL as `?authKey=` |

The dialled `serviceCode` is kept in the session data as `service_code`.

## Management API

`/api/v1/...` admits **internal service accounts only**, and every resource
belongs to the account that created it:

- Menus (`/menus`): created under the caller's ownership; reading, changing,
  translating or deleting another owner's menu answers 404.
- Service settings (`/services/{id}/config`): the first account to configure a
  service owns it; others get 403 on change and 404 on read (the auth key is
  among them).
- Caller data (`PUT /queries`): `{name, contact_id, profile_id, rows:[{payload}]}`
  replaces, in one step, what the caller's menus show that contact under
  `name`. An empty `rows` clears it, as does `DELETE /queries {name, contact_id}`.
  A menu reads only rows its own owner wrote, so services never see each
  other's data.

## Completed sessions

When a flow completes, the engine sends the notification service an inbound
notification: `type` `ussd`, `data` the service id, `source` the caller's
`profile_id` and `contact_id` (never the number), and `payload` the session
data. The notification service publishes it on the receive route for `ussd`,
where the owning service consumes it. A failed hand-off is retried.

## Setup

`setup` (or `DO_SETUP=true`) runs the database migration and exits; the
runtime never migrates.
