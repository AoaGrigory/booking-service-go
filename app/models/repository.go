package models

import (
	"context"
	"time"
)

// BookingRepository -- интерфейс репозитория бронирований.
type BookingRepository interface {
	// Create сохраняет новое бронирование и возвращает присвоенный ID.
	Create(ctx context.Context, booking *Booking) (int64, error)

	// GetByID возвращает бронирование по ID.
	GetByID(ctx context.Context, id int64) (*Booking, error)

	// Update обновляет бронирование в хранилище.
	Update(ctx context.Context, booking *Booking) error

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
