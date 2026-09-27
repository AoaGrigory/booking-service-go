package handlers

import (
	"booking-service/app/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"booking-service/app/messaging"
	"booking-service/app/service"
)

// BookingConfirmedHandler обрабатывает события BookingJobConfirmed.
type BookingConfirmedHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
}

// NewBookingConfirmedHandler создаёт новый обработчик.
func NewBookingConfirmedHandler(svc *service.BookingsService, logger *zap.Logger) *BookingConfirmedHandler {
	return &BookingConfirmedHandler{
		service: svc,
		logger:  logger,
	}
}

// Handle обрабатывает событие подтверждения бронирования.
func (h *BookingConfirmedHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.BookingJobConfirmed
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация BookingJobConfirmed: %w", err)
	}

	if event.EventId == "" {
		return fmt.Errorf("eventID пустой")
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие BookingJobConfirmed",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
	)
	check, err := h.service.CheckProcessEvent(ctx, event.EventId)
	if err != nil {
		return fmt.Errorf("проверка идемпотентности %s: %w", event.EventId, err)
	}
	if check {
		h.logger.Warn("дубликат подтверждения бронирования",
			zap.Int64("bookingID", bookingID),
			zap.String("eventID", event.EventId))
		return nil
	}
	if err := h.service.ConfirmFromEvent(ctx, bookingID, event.EventId); err != nil {
		if errors.Is(err, models.ErrProcessEvent) {
			h.logger.Warn("дубликат подтверждения бронирования",
				zap.Int64("bookingID", bookingID),
				zap.String("eventID", event.EventId))
			return nil
		}
		return fmt.Errorf("подтверждение бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("бронирование подтверждено через событие", zap.Int64("bookingId", bookingID))
	return nil
}
