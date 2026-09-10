-- +goose Up
CREATE TABLE IF NOT EXISTS booking_history (
    id                   BIGSERIAL    PRIMARY KEY,
    booking_id           BIGINT       NOT NULL REFERENCES bookings(id),
    previous_status      VARCHAR(30),
    new_status           VARCHAR(30)  NOT NULL,
    changed_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    reason               VARCHAR(100) NOT NULL,
    initiator            VARCHAR(30)  NOT NULL
    );


-- +goose Down
DROP TABLE IF EXISTS booking_history;