package models

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type OutboxRepository interface {
	AddTx(ctx context.Context, tx pgx.Tx, outboxMessage OutboxMessage) error

	BeginTx(ctx context.Context) (pgx.Tx, error)

	GetPending(ctx context.Context, tx pgx.Tx, limit int) ([]OutboxMessage, error)

	MarkSent(ctx context.Context, tx pgx.Tx, id int64) error

	RecordFailure(ctx context.Context, tx pgx.Tx, id int64, lastError string, maxAttempts int) error
}
