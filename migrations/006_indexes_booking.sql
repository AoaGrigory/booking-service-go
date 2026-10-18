-- +goose Up
-- Индекс для поиска зависших отмен
CREATE INDEX idx_status_and_cancellation_sent_at ON bookings(status, cancellation_sent_at);

-- Индекс для статистики за период
CREATE INDEX idx_bookings_created_at ON bookings(created_at);

CREATE INDEX idx_status_and_created_at ON bookings(status, created_at);

CREATE INDEX idx_outbox_messages_status_created_at ON outbox_messages (status, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_status_and_cancellation_sent_at;

DROP INDEX IF EXISTS idx_bookings_created_at;

DROP INDEX IF EXISTS idx_status_and_created_at;

DROP INDEX IF EXISTS idx_outbox_messages_status_created_at;
