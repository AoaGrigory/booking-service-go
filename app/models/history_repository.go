package models

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type HistoryRepository interface {

	// AddHistoryTx сохраняет в историю
	AddHistoryTx(ctx context.Context, tx pgx.Tx, entry *BookingHistory) error

	// GetBookingHistory возвращает историю изменения статуса
	GetBookingHistory(ctx context.Context, bookingID int64, limit, offset int) ([]BookingHistory, error)

	// GetBookingHistoryCount возвращает количество бронирований из истории
	GetBookingHistoryCount(ctx context.Context, bookingID int64) (int64, error)
}
