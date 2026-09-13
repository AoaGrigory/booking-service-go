package postgres

import (
	"booking-service/app/models"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func (r *BookingsRepository) GetBookingHistory(ctx context.Context, bookingId int64, limit, offset int) ([]models.BookingHistory, error) {

	rows, err := r.pool.Query(ctx, queryGetBookingHistoryByID, bookingId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("проверка поиска истории по статусу бронирования: %w", err)
	}
	var history []models.BookingHistory
	defer rows.Close()
	for rows.Next() {
		var (
			id             int64
			bookingID      int64
			previousStatus *string
			newStatus      string
			changedAt      time.Time
			reason         *string
			initiator      string
		)
		err := rows.Scan(&id, &bookingID, &previousStatus, &newStatus, &changedAt, &reason, &initiator)
		if err != nil {
			return nil, fmt.Errorf("ошибка в GetBookingHistory при чтении %d: %w", bookingId, err)
		}
		var ps *models.BookingStatus
		if previousStatus != nil {
			status := models.BookingStatus(*previousStatus)
			ps = &status
		}
		var reasonValue string
		if reason != nil {
			reasonValue = *reason
		}

		history = append(history, *models.RestoreBookingHistory(id, bookingID, ps, models.BookingStatus(newStatus), changedAt, reasonValue, initiator))
	}

	return history, rows.Err()
}

func (r *BookingsRepository) GetBookingHistoryCount(ctx context.Context, bookingID int64) (int64, error) {
	var count int64

	row := r.pool.QueryRow(ctx, queryGetCountBookingHistory, bookingID)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("получение количества записей истории бронирования: %w", err)
	}

	return count, nil
}

func (r *BookingsRepository) AddHistoryTx(ctx context.Context, tx pgx.Tx, entry *models.BookingHistory) error {

	var previousStatus *string
	if entry.PreviousStatus() != nil {
		s := string(*entry.PreviousStatus())
		previousStatus = &s
	}
	if _, err := tx.Exec(ctx, queryInsertBookingHistory,
		entry.BookingID(),
		previousStatus,
		string(entry.NewStatus()),
		entry.ChangedAt(),
		entry.Reason(),
		entry.Initiator(),
	); err != nil {
		return fmt.Errorf("ошибка добавления в историю: %w", err)
	}

	return nil
}
