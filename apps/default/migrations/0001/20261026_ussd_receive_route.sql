-- Copyright 2023-2026 Ant Investor Ltd
-- Completed USSD sessions are delivered to the service that owns the menu.
--
-- The USSD app hands a completed session to Receive as an inbound "ussd"
-- notification from the caller (profile and contact id). Seed is the only
-- service with a USSD menu, so the receive route for ussd publishes to its
-- inbound topic in stawi-finance; Seed consumes it there.
INSERT INTO routes(id, tenant_id, partition_id, name, description, mode, route_type, uri, created_at, modified_at)
VALUES ('d4eussd0rcv0seed0001', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg',
        'Seed USSD Receive', 'Completed Seed USSD sessions, delivered to the seed service', 'rx', 'ussd',
        'gcppubsub://stawi-finance/finance-seed-ussd-inbound',
        '2026-10-26 06:00:00', '2026-10-26 06:00:00')
ON CONFLICT (id) DO NOTHING;
