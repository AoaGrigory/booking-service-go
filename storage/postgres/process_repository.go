package postgres

import (
	"booking-service/app/models"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *BookingsRepository) AddProcessTx(ctx context.Context, tx pgx.Tx, eventID string) error {
	if _, err := tx.Exec(ctx, queryInsertEventID, eventID); err != nil {
		if isUniqueViolation(err) {
			return models.ErrProcessEvent
		}
		return fmt.Errorf("добавление обработанного события: %w", err)
	}
	return nil
}

func (r *BookingsRepository) CheckProcess(ctx context.Context, eventID string) (bool, error) {
	var check bool
	row := r.pool.QueryRow(ctx, queryCheckProcessByEventID, eventID)
	if err := row.Scan(&check); err != nil {
		return false, fmt.Errorf("ошибка проверки обработанного события: %w", err)
	}
	return check, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
