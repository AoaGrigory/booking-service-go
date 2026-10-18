package worker

import (
	"booking-service/app/messaging"
	"booking-service/app/models"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"time"
)

type OutboxWorker struct {
	repo        models.OutboxRepository
	publisher   *messaging.Publisher
	interval    time.Duration
	maxAttempts int
	batchSize   int
	logger      *zap.Logger
}

func NewOutboxWorker(repo models.OutboxRepository, publisher *messaging.Publisher, interval time.Duration, maxAttempts, batchSize int, logger *zap.Logger) *OutboxWorker {
	return &OutboxWorker{
		repo:        repo,
		publisher:   publisher,
		interval:    interval,
		maxAttempts: maxAttempts,
		batchSize:   batchSize,
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
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		w.logger.Error("начало транзакции outbox", zap.Error(err))
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	messages, err := w.repo.GetPending(ctx, tx, w.batchSize)
	if err != nil {
		w.logger.Error("ошибка получения сообщения outbox", zap.Error(err))
		return
	}
	if len(messages) == 0 {
		return
	}
	success, failed := 0, 0
	for _, mes := range messages {
		if ctx.Err() != nil {
			return
		}
		sent, err := w.processMessage(ctx, tx, mes)
		if err != nil {
			w.logger.Error("сохранение результата обработки outbox; транзакция пачки будет отменена",
				zap.Error(err),
				zap.Int64("outboxID", mes.ID))
			return
		}
		if sent {
			success++
		} else {
			failed++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		w.logger.Error("коммит результатов обработки outbox", zap.Error(err))
		return
	}
	w.logger.Info("проход обработки outbox завершён",
		zap.Int("success", success), zap.Int("failed", failed))

}

func (w *OutboxWorker) processMessage(ctx context.Context, tx pgx.Tx, message models.OutboxMessage) (bool, error) {
	var event messaging.BookingStatusChangedEvent
	err := json.Unmarshal(message.Payload, &event)
	if err == nil {
		err = w.publisher.PublishBookingStatusChanged(ctx, event)
	}
	if err != nil {
		w.logger.Error("не удалось отправить сообщение outbox",
			zap.Int64("outboxID", message.ID), zap.Error(err))
		saveErr := w.repo.RecordFailure(ctx, tx, message.ID, err.Error(), w.maxAttempts)
		if saveErr != nil {
			return false, fmt.Errorf("сохранение неудачной попытки (%v): %w", err, saveErr)
		}

		return false, nil
	}
	err = w.repo.MarkSent(ctx, tx, message.ID)
	if err != nil {

		return false, fmt.Errorf("отметка сообщения outbox как отправленного: %w", err)
	}

	return true, nil
}
