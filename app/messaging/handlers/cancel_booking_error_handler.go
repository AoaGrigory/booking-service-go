package handlers

import (
	"booking-service/app/messaging"
	"booking-service/app/models"
	"booking-service/app/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
)

type BookingErrorHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
}

func NewBookingErrorHandler(svc *service.BookingsService, logger *zap.Logger) *BookingErrorHandler {
	return &BookingErrorHandler{
		service: svc,
		logger:  logger,
	}
}
func (h *BookingErrorHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.CancelBookingJobCommand
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация BookingErrorHandler: %w", err)
	}
	if event.EventId == "" {
		return fmt.Errorf("eventID пустой")
	}
	check, err := h.service.CheckProcessEvent(ctx, event.EventId)
	if err != nil {
		return fmt.Errorf("проверка идемпотентности %s: %w", event.EventId, err)
	}
	if check {
		h.logger.Warn("дубликат подтверждения бронирования ",
			zap.String("eventID", event.EventId))
		return nil
	}

	if err := h.service.HandleCancelErrorFromEvent(ctx, event.EventId, event.RequestId); err != nil {
		if errors.Is(err, models.ErrProcessEvent) {
			h.logger.Warn("дубликат события ошибки отмены",
				zap.String("eventID", event.EventId))
			return nil
		}
		return fmt.Errorf("обработка ошибки отмены события %s: %w", event.EventId, err)

	}
	h.logger.Info("получено сообщение об ошибке отмены",
		zap.String("requestId", event.RequestId))

	return nil

}
