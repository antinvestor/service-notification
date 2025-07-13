INSERT INTO routes(id, tenant_id, partition_id, name, description, mode, route_type, uri, created_at, modified_at)
VALUES ('9bsv0s5943v0036a8sp0', '9bsv0s0hijjg02qks6dg', '9bsv0s0hijjg02qks6i0',
        'Stawi Dev Receive', 'Channel to receive stawi dev messages', 'rx', 'any',
        'nats://core_notification:P99NW58uZy4z@transactional-db.core:4222?subject=ROUTED.STAWI.DEV.MESSAGING.SOURCE.MESSAGING',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s5943v0036a8tp5', '9bsv0s0hijjg02qks6dg', '9bsv0s0hijjg02qks6i0',
        'Stawi Dev Send', 'Channel to send stawi dev messages', 'tx', 'any',
        'nats://core_notification:P99NW58uZy4z@transactional-db.core:4222?subject=ROUTED.REALTIME.OUTBOUND.MESSAGES',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s0d0smg0338lcvg', '9bsv0s0hijjg02qks6jg', '9bsv0s0hijjg02qks6kg',
        'Stawi Production Receive', 'Channel to receive stawi messages', 'rx', 'any',
        'nats://core_notification:P99NW58uZy4z@transactional-db.core:4222?subject=ROUTED.STAWI.DEV.MESSAGING.SOURCE.MESSAGING',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s0d0smg0339ldvh', '9bsv0s0hijjg02qks6jg', '9bsv0s0hijjg02qks6kg',
        'Stawi Production Send', 'Channel to send stawi messages', 'tx', 'any',
        'nats://core_notification:P99NW58uZy4z@transactional-db.core:4222?subject=ROUTED.REALTIME.OUTBOUND.MESSAGES',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30');

