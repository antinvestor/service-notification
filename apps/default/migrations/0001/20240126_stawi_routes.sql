INSERT INTO routes(id, tenant_id, partition_id, name, description, mode, route_type, uri, created_at, modified_at)
VALUES ('9bsv0s5943v0036a8sp0', '9bsv0s0hijjg09bzz6dg', '9bsv0s0hijjg02qks6i0',
        'Stawi Dev Receive', 'Channel to send/receive stawi dev email messages', 'tx', 'email',
        'nats://core-queue-headless.queue-system.svc.cluster.local:4222?subject=svc.notification.integration.emailsmtp.send.queue&jetstream=true',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s5943v0036a8tp5', '9bsv0s0hijjg09bzz6dg', '9bsv0s0hijjg02qks6i0',
        'Stawi Dev Send', 'Channel to send stawi dev sms messages', 'tx', 'sms',
        'nats://core-queue-headless.queue-system.svc.cluster.local:4222?subject=svc.notification.integration.africastalking.send.queue&jetstream=true',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s0d0smg0338lcvg', '9bsv0s0hijjg02z5lbjg', '9bsv0s0hijjg02qk7l1g',
        'Stawi Production Receive', 'Channel to send/receive stawi email messages', 'tx', 'email',
        'nats://core-queue-headless.queue-system.svc.cluster.local:4222?subject=svc.notification.integration.emailsmtp.send.queue&jetstream=true',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30'),
       ('9bsv0s0d0smg0339ldvh', '9bsv0s0hijjg02z5lbjg', '9bsv0s0hijjg02qk7l1g',
        'Stawi Production Send', 'Channel to send stawi sms messages', 'tx', 'sms',
        'nats://core-queue-headless.queue-system.svc.cluster.local:4222?subject=svc.notification.integration.africastalking.send.queue&jetstream=true',
        '2024-01-15 07:45:30', '2024-01-15 07:45:30');

