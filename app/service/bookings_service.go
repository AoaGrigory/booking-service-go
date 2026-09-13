package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"booking-service/app/api/dto"
	"booking-service/app/messaging"
	"booking-service/app/models"
)

// BookingsService обрабатывает команды (изменение состояния) для бронирований.
//
// Этот сервис -- оркестратор: он координирует домен и репозиторий,
// но НЕ содержит бизнес-правила (они в models.Booking).
const InitiatorSystem = "System"

type BookingsService struct {
	repo      models.BookingRepository
	publisher *messaging.Publisher
	logger    *zap.Logger
}

// NewBookingsService создаёт новый BookingsService.
func NewBookingsService(repo models.BookingRepository, publisher *messaging.Publisher, logger *zap.Logger) *BookingsService {
	return &BookingsService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
	}
}

// Create создаёт новое бронирование.
//
// Шаги:
//  1. Парсинг дат из строкового формата
//  2. Создание доменного объекта (валидация в конструкторе)
//  3. Сохранение в БД
//  4. Публикация команды в Catalog
//  5. Возврат ID
func (s *BookingsService) Create(ctx context.Context, req dto.CreateBookingRequest) (int64, error) {
	startDate, err := time.Parse(dto.DateFormat, req.StartDate)
	if err != nil {
		return 0, fmt.Errorf("некорректный формат startDate: %w", err)
	}

	endDate, err := time.Parse(dto.DateFormat, req.EndDate)
	if err != nil {
		return 0, fmt.Errorf("некорректный формат endDate: %w", err)
	}

	booking, err := models.NewBooking(req.UserID, req.ResourceID, startDate, endDate)
	if err != nil {
		return 0, err
	}
	reason := "Booking created by user"
	initiator := strconv.FormatInt(booking.UserID(), 10)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("ошибка начала транзакции в Create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := s.repo.CreateTx(ctx, tx, booking)
	if err != nil {
		return 0, fmt.Errorf("сохранение бронирования: %w", err)
	}

	entry, err := models.NewBookingHistory(
		id,
		nil,
		booking.Status(),
		time.Now().UTC(),
		reason,
		initiator,
	)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания истории в Create: %w", err)
	}
	if err := s.repo.AddHistoryTx(ctx, tx, entry); err != nil {
		return 0, fmt.Errorf("ошибка транзакции при добавлении в историю в Create: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("ошибка коммита транзакции в Create: %w", err)
	}

	s.logger.Info("бронирование создано",
		zap.Int64("id", id),
		zap.Int64("userId", req.UserID),
		zap.Int64("resourceId", req.ResourceID),
	)

	if err := s.publisher.PublishCreateBookingJob(ctx, messaging.CreateBookingJobCommand{
		EventId:    messaging.NewMessageID(),
		RequestId:  messaging.BookingIDToRequestID(id),
		ResourceId: req.ResourceID,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
	}); err != nil {
		s.logger.Error("ошибка публикации CreateBookingJob", zap.Error(err), zap.Int64("bookingId", id))
		// Не возвращаем ошибку -- бронирование уже создано, команда может быть обработана позже
	}

	return id, nil
}

// Cancel отменяет бронирование по ID.
//
// Шаги:
//  1. Загрузка бронирования из БД
//  2. Вызов доменного метода Cancel() (валидация перехода статуса)
//  3. Сохранение обновлённого состояния
//  4. Публикация команды в Catalog
func (s *BookingsService) Cancel(ctx context.Context, id int64) error {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	oldStatus := booking.Status()
	reason := "user requested cancellation"
	initiator := strconv.FormatInt(booking.UserID(), 10)
	if err := booking.StartCancellation(time.Now()); err != nil {
		return err
	}
	entry, err := models.NewBookingHistory(
		booking.ID(),
		&oldStatus,
		booking.Status(),
		time.Now().UTC(),
		reason,
		initiator,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания истории в Cancel: %w", err)
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции в Cancel: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.UpdateTx(ctx, tx, booking); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}
	if err := s.repo.AddHistoryTx(ctx, tx, entry); err != nil {
		return fmt.Errorf("добавление в историю в Cancel: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка при коммите в Cancel: %w", err)
	}

	if err := s.publisher.PublishCancelBookingJob(ctx, messaging.CancelBookingJobCommand{
		EventId:   messaging.NewMessageID(),
		RequestId: messaging.BookingIDToRequestID(id),
	}); err != nil {
		s.logger.Error("ошибка публикации CancelBookingJob", zap.Error(err), zap.Int64("bookingId", id))
	}

	return nil
}

// Confirm подтверждает бронирование по ID.
// Используется обработчиком событий RabbitMQ.
func (s *BookingsService) Confirm(ctx context.Context, id int64) error {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if booking.Status() == models.BookingStatusCancellationPending {
		s.logger.Warn("Обнаружена race condition: переход cancellation_pending -> confirmed",
			zap.Int64("bookingId", id))
	}
	oldStatus := booking.Status()
	reason := "confirm from Catalog"
	initiator := InitiatorSystem
	if err := booking.Confirm(); err != nil {
		return err
	}
	entry, err := models.NewBookingHistory(
		booking.ID(),
		&oldStatus,
		booking.Status(),
		time.Now().UTC(),
		reason,
		initiator,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания истории в confirm: %w", err)
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции в confirm: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.UpdateTx(ctx, tx, booking); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}
	if err := s.repo.AddHistoryTx(ctx, tx, entry); err != nil {
		return fmt.Errorf("добавление в историю в confirm: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка при коммите в confirm: %w", err)
	}

	s.logger.Info("бронирование подтверждено", zap.Int64("id", id))

	return nil
}

// HandleCancelError запускает роллбэк статуса
func (s *BookingsService) HandleCancelError(ctx context.Context, requestID string) error {
	bookingId, err := messaging.RequestIDToBookingID(requestID)
	if err != nil {
		return fmt.Errorf("некорректный requestID %s: %w", requestID, err)
	}
	booking, err := s.repo.GetByID(ctx, bookingId)
	if err != nil {
		return err
	}

	oldStatus := booking.Status()
	reason := "cancellation rollback after error"
	initiator := InitiatorSystem
	if err := booking.RollbackCancellation(); err != nil {
		return fmt.Errorf("роллбэк: %w", err)
	}
	entry, err := models.NewBookingHistory(
		booking.ID(),
		&oldStatus,
		booking.Status(),
		time.Now().UTC(),
		reason,
		initiator,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания истории в HandleCancelError: %w", err)
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции в HandleCancelError: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.UpdateTx(ctx, tx, booking); err != nil {
		return fmt.Errorf("обновление при роллбэке: %w", err)
	}
	if err := s.repo.AddHistoryTx(ctx, tx, entry); err != nil {
		return fmt.Errorf("ошибка транзакции в HandleCancelError: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка при коммите в HandleCancelError: %w", err)
	}
	s.logger.Info("Успешный роллбэк, статус возвращен",
		zap.Int64("id", bookingId),
		zap.String("status", string(booking.Status())))

	return nil
}

// CompleteCancellation завершает отмену: cancellation_pending -> cancelled
func (s *BookingsService) CompleteCancellation(ctx context.Context, id int64) error {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("получение бронирования при завершении отмены %d: %w", id, err)
	}
	oldStatus := booking.Status()
	reason := "cancellation confirmed by Catalog"
	initiator := InitiatorSystem
	if err := booking.CompleteCancellation(); err != nil {
		return fmt.Errorf("завершение отмены бронирования %d: %w", id, err)
	}
	entry, err := models.NewBookingHistory(
		booking.ID(),
		&oldStatus,
		booking.Status(),
		time.Now().UTC(),
		reason,
		initiator,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания истории в CompleteCancellation: %w", err)
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции в CompleteCancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.UpdateTx(ctx, tx, booking); err != nil {
		return fmt.Errorf("обновление при отмене: %w", err)
	}
	if err := s.repo.AddHistoryTx(ctx, tx, entry); err != nil {
		return fmt.Errorf("ошибка транзакции в CompleteCancellation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка при коммите в CompleteCancellation: %w", err)
	}
	s.logger.Info("успешная отмена, статус изменен",
		zap.Int64("id", id),
		zap.String("status", string(booking.Status())))

	return nil
}
