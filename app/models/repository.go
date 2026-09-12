package models

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// BookingRepository -- интерфейс репозитория бронирований.
type BookingRepository interface {
	Create(ctx context.Context, booking *Booking) (int64, error)

	// GetByID возвращает бронирование по ID.
	GetByID(ctx context.Context, id int64) (*Booking, error)

	// GetByFilter возвращает список бронирований с пагинацией.
	GetByFilter(ctx context.Context, filter BookingFilter) ([]Booking, int64, error)

	// GetAwaitingConfirmation возвращает бронирования в статусе AwaitsConfirmation
	// с пессимистичной блокировкой (SELECT ... FOR UPDATE SKIP LOCKED).
	GetAwaitingConfirmation(ctx context.Context, limit int) ([]Booking, error)

	// GetOrdersPerPeriod возвращает все бронирования за указанный интервал времени
	GetOrdersPerPeriod(ctx context.Context, dateFrom, dateTo time.Time) (int64, error)

	// GetOrdersByStatus возвращает статистику статусов всех бронирований
	// за указанный промежуток времени
	GetOrdersByStatus(ctx context.Context, dateFrom, dateTo time.Time) (map[string]int64, error)

	// GetOrdersTopFiveResource возвращает топ 5 используемых ресурсов
	// и количество бронирований, которые используют эти ресурсы
	GetOrdersTopFiveResource(ctx context.Context, dateFrom, dateTo time.Time) ([]ResourceBookingStats, error)

	// GetBookingsWithStatusCancellationPending возвращает бронирования в статусе CancellationPending
	GetBookingsWithStatusCancellationPending(ctx context.Context, threshold time.Time, limit int) ([]Booking, error)

	// GetBookingHistory возвращает историю изменения статуса
	GetBookingHistory(ctx context.Context, bookingID int64, limit, offset int) ([]BookingHistory, error)

	// GetBookingHistoryCount возвращает количество бронирований из истории
	GetBookingHistoryCount(ctx context.Context, bookingID int64) (int64, error)

	// CreateTx сохраняет новое бронирование,возвращает присвоенный ID
	// и сохраняет в историю
	CreateTx(ctx context.Context, tx pgx.Tx, booking *Booking) (int64, error)

	// UpdateTx обновляет бронирование в хранилище
	// и сохраняет в историю
	UpdateTx(ctx context.Context, tx pgx.Tx, booking *Booking) error

	// AddHistoryTx сохраняет в историю
	AddHistoryTx(ctx context.Context, tx pgx.Tx, entry *BookingHistory) error

	// BeginTx начинает транзакцию
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// BookingFilter содержит параметры фильтрации и пагинации.
type BookingFilter struct {
	UserID     *int64
	ResourceID *int64
	Status     *BookingStatus
	Page       int
	Size       int
}

// NewDefaultFilter создаёт фильтр с пагинацией по умолчанию.
func NewDefaultFilter() BookingFilter {
	return BookingFilter{
		Page: 1,
		Size: 25,
	}
}
