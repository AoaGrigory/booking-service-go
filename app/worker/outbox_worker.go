package worker

import (
	"booking-service/app/messaging"
	"booking-service/app/models"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"time"
)

type OutboxWorker struct {
	repo        models.OutboxRepository
	publisher   *messaging.Publisher
	interval    time.Duration
	maxAttempts int
	logger      *zap.Logger
}

func NewOutboxWorker(repo models.OutboxRepository, publisher *messaging.Publisher, interval time.Duration, maxAttempts int, logger *zap.Logger) *OutboxWorker {
	return &OutboxWorker{
		repo:        repo,
		publisher:   publisher,
		interval:    interval,
		maxAttempts: maxAttempts,
		logger:      logger,
	}
}
func (w *OutboxWorker) Run(ctx context.Context) {
	w.logger.Info("воркер outbox запущен",
		zap.Duration("interval", w.interval),
		zap.Int("maxAttempts", w.maxAttempts),
	)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("воркер outbox остановлен")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}

	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	messages, err := w.repo.GetPending(ctx)
	if err != nil {
		w.logger.Error("ошибка получения сообщения outbox", zap.Error(err))
		return
	}
	for _, mes := range messages {
		if err := w.processMessage(ctx, mes); err != nil {
			w.logger.Error("ошибка при итерации message в processBatch",
				zap.Error(err),
				zap.Int64("outboxID", mes.ID))
		}
	}
	w.logger.Info("проход обработки outbox завершён")

}

func (w *OutboxWorker) processMessage(ctx context.Context, message models.OutboxMessage) error {
	var event messaging.BookingStatusChangedEvent
	err := json.Unmarshal(message.Payload, &event)
	if err != nil {
		saveErr := w.repo.RecordFailure(ctx, message.ID, err.Error(), w.maxAttempts)
		if saveErr != nil {
			return saveErr
		}
		return fmt.Errorf("ошибка в processMessage: %w", err)
	}

	err = w.publisher.PublishBookingStatusChanged(ctx, event)
	if err != nil {
		saveErr := w.repo.RecordFailure(ctx, message.ID, err.Error(), w.maxAttempts)
		if saveErr != nil {
			return saveErr
		}
		return fmt.Errorf("ошибка при публикации в processMessage: %w", err)
	}
	err = w.repo.MarkSent(ctx, message.ID)
	if err != nil {

		return fmt.Errorf("отметка сообщения outbox как отправленного: %w", err)
	}

	return nil
}
