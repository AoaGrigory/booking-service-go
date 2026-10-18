package postgres

import (
	"booking-service/app/models"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) AddTx(ctx context.Context, tx pgx.Tx, outboxMessage models.OutboxMessage) error {
	if _, err := tx.Exec(ctx, queryInsertOutboxMessage, outboxMessage.Payload); err != nil {
		return fmt.Errorf("ошибка при добавлении в outbox: %w", err)
	}
	return nil
}

func (r *OutboxRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *OutboxRepository) GetPending(ctx context.Context, tx pgx.Tx, limit int) ([]models.OutboxMessage, error) {
	rows, err := tx.Query(ctx, queryGetOutboxMessage, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении outbox: %w", err)
	}
	defer rows.Close()
	var messages []models.OutboxMessage

	for rows.Next() {
		var message models.OutboxMessage

		err := rows.Scan(
			&message.ID,
			&message.Payload,
			&message.Status,
			&message.Attempts,
			&message.CreatedAt,
			&message.SentAt,
			&message.LastError,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка в GetPending при чтении: %w", err)
		}
		messages = append(messages, message)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("ошибка итерации GetPending: %w", rows.Err())
	}
	return messages, nil
}

func (r *OutboxRepository) MarkSent(ctx context.Context, tx pgx.Tx, id int64) error {

	_, err := tx.Exec(ctx, queryUpdateOutboxMessage, id)
	if err != nil {
		return fmt.Errorf("ошибка в MarkSent при апдейте outbox: %w", err)
	}

	return nil
}

func (r *OutboxRepository) RecordFailure(ctx context.Context, tx pgx.Tx, id int64, lastError string, maxAttempts int) error {

	_, err := tx.Exec(ctx, queryRecordFailureOutboxMessage, id, lastError, maxAttempts)
	if err != nil {
		return fmt.Errorf("ошибка в RecordFailure при апдейте outbox: %w", err)
	}

	return nil
}
