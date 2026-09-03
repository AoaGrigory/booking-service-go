package handlers

import (
	"booking-service/app/messaging"
	"booking-service/app/service"
	"context"
	"encoding/json"
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

	if err := h.service.HandleCancelError(ctx, event.RequestId); err != nil {
		return fmt.Errorf("rollback бронирования %s: %w", event.RequestId, err)
	}
	h.logger.Info("получено сообщение об ошибке отмены",
		zap.String("requsetId", event.RequestId))

	return nil

}
