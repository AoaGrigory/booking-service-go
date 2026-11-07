-- +goose Up
CREATE TABLE IF NOT EXISTS processed_events(
    event_id TEXT UNIQUE NOT NULL);

-- +goose Down
DROP TABLE IF EXISTS processed_events;