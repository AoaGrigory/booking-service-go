package worker

import (
	"booking-service/app/messaging"
	"booking-service/app/models"
	"booking-service/app/service"
	"context"
	"go.uber.org/zap"
	"time"
)

// CancellationRetryWorker -- фоновый воркер для проверки статуса cancellation_pending

type CancellationRetryWorker struct {
	service   *service.BookingsService
	repo      models.BookingRepository
	message   messaging.Publisher
	timeout   time.Duration
	interval  time.Duration
	batchSize int
	logger    *zap.Logger
}

// NewCancellationRetryWorker создает новый воркер проверки cancellation_pending
func NewCancellationRetryWorker(
	svc *service.BookingsService,
	repo models.BookingRepository,
	message messaging.Publisher,
	timeout time.Duration,
	interval time.Duration,
	batchSize int,
	logger *zap.Logger,
) *CancellationRetryWorker {
	return &CancellationRetryWorker{
		service:   svc,
		repo:      repo,
		message:   message,
		timeout:   timeout,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger,
	}
}

func (w *CancellationRetryWorker) Run(ctx context.Context) {
	w.logger.Info("воркер проверки cancellation_pending запущен",
		zap.Duration("interval", w.interval),
		zap.Int("batchSize", w.batchSize),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("воркер проверки cancellation_pending остановлен")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *CancellationRetryWorker) processBatch(ctx context.Context) {
	threshold := time.Now().Add(-w.timeout)
	bookings, err := w.repo.GetBookingsWithStatusCancellationPending(ctx, threshold, w.batchSize)
	if err != nil {
		w.logger.Error("ошибка получения бронирования для поиска зависших", zap.Error(err))
		return
	}
	if len(bookings) == 0 {
		return
	}
	w.logger.Info("обработка бронирования", zap.Int("count", len(bookings)))

	for _, booking := range bookings {
		w.processBooking(ctx, &booking)
	}
}

func (w *CancellationRetryWorker) processBooking(ctx context.Context, booking *models.Booking) {
	bookingID := booking.ID()
	cmd := messaging.CancelBookingJobCommand{
		EventId:   messaging.NewMessageID(),
		RequestId: messaging.BookingIDToRequestID(bookingID),
	}
	logger := w.logger.With(zap.Int64("bookingID", bookingID))
	logger.Info("начало повторной отмены бронирования")
	if err := w.message.PublishCancelBookingJob(ctx, cmd); err != nil {
		logger.Error("ошибка при повторной отмене бронирования", zap.Error(err))
	} else {
		logger.Info("повторная отмена бронирования успешно")
	}

}
