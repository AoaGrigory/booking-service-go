package models

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type OutboxRepository interface {
	AddTx(ctx context.Context, tx pgx.Tx, outboxMessage OutboxMessage) error

	GetPending(ctx context.Context) ([]OutboxMessage, error)

	MarkSent(ctx context.Context, id int64) error

	RecordFailure(ctx context.Context, id int64, lastError string, maxAttempts int) error
}
