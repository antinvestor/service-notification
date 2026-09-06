-- Delivery reports from providers (Africa's Talking, WhatsApp) identify a message only by the
-- provider-assigned external id, so status updates resolve notifications through it.
CREATE INDEX IF NOT EXISTS idx_notifications_external_id ON notifications (external_id) WHERE external_id <> '';
