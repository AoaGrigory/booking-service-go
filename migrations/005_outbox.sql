-- +goose Up
CREATE TABLE IF NOT EXISTS outbox_messages (
    id         BIGSERIAL   PRIMARY KEY,
    payload    JSONB       NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts   INT         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMPTZ,
    last_error TEXT
);

-- +goose Down

DROP TABLE IF EXISTS outbox_messages;