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

// BookingDeniedHandler обрабатывает события BookingJobDenied.
type BookingDeniedHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
}

// NewBookingDeniedHandler создаёт новый обработчик.
func NewBookingDeniedHandler(svc *service.BookingsService, logger *zap.Logger) *BookingDeniedHandler {
	return &BookingDeniedHandler{
		service: svc,
		logger:  logger,
	}
}

// Handle обрабатывает событие отклонения бронирования.
func (h *BookingDeniedHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.BookingJobDenied
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация BookingJobDenied: %w", err)
	}

	if event.EventId == "" {
		return fmt.Errorf("eventID пустой")
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие BookingJobDenied",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
		zap.String("reason", event.Reason),
	)
	check, err := h.service.CheckProcessEvent(ctx, event.EventId)
	if err != nil {
		return fmt.Errorf("проверка идемпотентности %s: %w", event.EventId, err)
	}
	if check {
		h.logger.Warn("дубликат отмены бронирования",
			zap.Int64("bookingID", bookingID),
			zap.String("eventID", event.EventId))
		return nil
	}

	if err := h.service.CancelFromEvent(ctx, bookingID, event.EventId); err != nil {
		if errors.Is(err, models.ErrProcessEvent) {
			h.logger.Warn("дубликат отмены бронирования",
				zap.Int64("bookingID", bookingID),
				zap.String("eventID", event.EventId))
			return nil
		}
		return fmt.Errorf("запущена отмена бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("бронирование отменено через событие",
		zap.Int64("bookingId", bookingID),
		zap.String("reason", event.Reason),
	)
	return nil
}
