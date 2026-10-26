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

`POST /ussd/gateway/{serviceID}[/{protocol}]` (also `/gateway/...`, which is
what arrives through the API gateway's `/ussd` prefix) takes form, JSON or XML. Every
service must have an `auth_key`, and it must match `authKey` (form field,
query parameter or `X-Auth-Key` header): the engine acts for whoever the
number names, so a service without a key serves nobody. `whitelist_ip`
limits the source addresses.

For Africa's Talking, configure the service with:

| setting | value |
|---|---|
| `input_separator` | `*` (AT resends the whole path, `1*2*3`) |
| `data_begin_template`, `data_continue_template` | `CON %s` |
| `data_end_template` | `END %s` |
| `auth_key` | a secret, put in the callback URL as `?authKey=` |

The dialled `serviceCode` is kept in the session data as `service_code`.

## Caller state at session start

A service may set `session_start_url`. When a caller with a profile opens a
session, the engine POSTs `{service_id, profile_id, contact_id}` there as its
own service account and stores the answer, `{"rows": {name: [payload, ...]}}`,
as that caller's rows before the first screen is drawn: a name with no rows
clears them, a name left out keeps them. The URL must be https on a host in
`USSD_TRUSTED_SERVICE_HOSTS` (default `api.stawi.org`), because the call
carries the engine's token. If the call fails, the session runs on the rows
already stored.

## Secret input

A menu with `is_secret` (a PIN prompt) never keeps what the caller typed in
the clear. The value is sealed (NaCl anonymous box) with the service's
`secret_box_public_key` setting before it is stored in the session or handed
on, as `sealed:<base64>`; only the service holding the private key can open
it. A secret prompt in a service without that key ends the session. Inputs
are never logged.

## Management API

`/api/v1/...` admits **internal service accounts only**, and every resource
belongs to the account that created it:

- Menus (`/menus`): created under the caller's ownership; reading, changing,
  translating or deleting another owner's menu answers 404.
- Service settings (`/services/{id}/config`): the first account to configure a
  brand-new service owns it; others get 403 on change and 404 on read (the
  auth key is among them). Settings with no recorded owner are nobody's and
  cannot be claimed. `initial_menu` must name one of the owner's own menus.
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
