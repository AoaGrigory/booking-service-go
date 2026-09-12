package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"booking-service/app/api/dto"
	"booking-service/app/models"
)

// BookingsQueries обрабатывает запросы (чтение данных) для бронирований.
type BookingsQueries struct {
	repo   models.BookingRepository
	logger *zap.Logger
}

// NewBookingsQueries создаёт новый BookingsQueries.
func NewBookingsQueries(repo models.BookingRepository, logger *zap.Logger) *BookingsQueries {
	return &BookingsQueries{
		repo:   repo,
		logger: logger,
	}
}

// GetByID возвращает бронирование по ID.
func (q *BookingsQueries) GetByID(ctx context.Context, id int64) (dto.BookingResponse, error) {
	booking, err := q.repo.GetByID(ctx, id)
	if err != nil {
		return dto.BookingResponse{}, err
	}

	return mapBookingToResponse(booking), nil
}

// GetStatus возвращает статус бронирования по ID.
func (q *BookingsQueries) GetStatus(ctx context.Context, id int64) (models.BookingStatus, error) {
	booking, err := q.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return booking.Status(), nil
}

// GetByFilter возвращает список бронирований с пагинацией.
func (q *BookingsQueries) GetByFilter(ctx context.Context, req dto.GetBookingsByFilterRequest) (dto.PagedResponse[dto.BookingResponse], error) {
	filter := models.NewDefaultFilter()

	if req.Page > 0 {
		filter.Page = req.Page
	}
	if req.Size > 0 {
		filter.Size = req.Size
	}
	if req.UserID != nil {
		filter.UserID = req.UserID
	}
	if req.ResourceID != nil {
		filter.ResourceID = req.ResourceID
	}
	if req.Status != nil {
		status := models.BookingStatus(*req.Status)
		if !status.IsValid() {
			return dto.PagedResponse[dto.BookingResponse]{}, fmt.Errorf("некорректный статус: %s", *req.Status)
		}
		filter.Status = &status
	}

	bookings, totalCount, err := q.repo.GetByFilter(ctx, filter)
	if err != nil {
		return dto.PagedResponse[dto.BookingResponse]{}, fmt.Errorf("получение бронирований: %w", err)
	}

	items := make([]dto.BookingResponse, 0, len(bookings))
	for i := range bookings {
		items = append(items, mapBookingToResponse(&bookings[i]))
	}

	return dto.PagedResponse[dto.BookingResponse]{
		Items:      items,
		TotalCount: totalCount,
		Page:       filter.Page,
		Size:       filter.Size,
	}, nil
}

func (q *BookingsQueries) GetStatistic(ctx context.Context, dateFrom, dateTo time.Time) (dto.BookingStatistic, error) {
	bookingCount, err := q.repo.GetOrdersPerPeriod(ctx, dateFrom, dateTo)
	if err != nil {
		return dto.BookingStatistic{}, fmt.Errorf("сервис статистики количества бронирований: %w", err)
	}
	statusInfo, err := q.repo.GetOrdersByStatus(ctx, dateFrom, dateTo)
	if err != nil {
		return dto.BookingStatistic{}, fmt.Errorf("сервис статистики статусы: %w", err)
	}
	topFive, err := q.repo.GetOrdersTopFiveResource(ctx, dateFrom, dateTo)
	if err != nil {
		return dto.BookingStatistic{}, fmt.Errorf("сервис статистики топ 5 ресурсов: %w", err)
	}
	topFiveDto := make([]dto.TopResource, 0, len(topFive))
	for _, item := range topFive {
		topFiveDto = append(topFiveDto, dto.TopResource{
			ResourceID:   item.ResourceID,
			BookingCount: item.BookingCount,
		})
	}

	return dto.BookingStatistic{
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		CountOrders:     bookingCount,
		StatusStatistic: statusInfo,
		TopFive:         topFiveDto,
	}, nil
}

func (q *BookingsQueries) GetHistory(ctx context.Context, bookingId int64, req dto.GetBookingHistoryRequest) (dto.PagedResponse[dto.BookingHistoryResponse], error) {
	page := req.Page
	size := req.Size
	offset := (page - 1) * size

	bookingHistory := make([]dto.BookingHistoryResponse, 0)
	count, err := q.repo.GetBookingHistoryCount(ctx, bookingId)
	if err != nil {
		return dto.PagedResponse[dto.BookingHistoryResponse]{}, err
	}

	booking, err := q.repo.GetBookingHistory(ctx, bookingId, size, offset)
	if err != nil {
		return dto.PagedResponse[dto.BookingHistoryResponse]{}, err
	}
	for _, val := range booking {
		historyDto := dto.BookingHistoryResponse{
			ID:             val.GetID(),
			BookingID:      val.BookingID(),
			PreviousStatus: string(val.PreviousStatus()),
			NewStatus:      string(val.NewStatus()),
			ChangedAt:      val.ChangedAt().Format(time.RFC3339),
			Reason:         val.Reason(),
			Initiator:      val.Initiator(),
		}
		bookingHistory = append(bookingHistory, historyDto)
	}

	return dto.PagedResponse[dto.BookingHistoryResponse]{
		Items:      bookingHistory,
		TotalCount: count,
		Page:       page,
		Size:       size}, nil
}

// mapBookingToResponse конвертирует доменный объект в DTO ответа.
func mapBookingToResponse(b *models.Booking) dto.BookingResponse {
	return dto.BookingResponse{
		ID:         b.ID(),
		Status:     string(b.Status()),
		UserID:     b.UserID(),
		ResourceID: b.ResourceID(),
		StartDate:  b.StartDate().Format(dto.DateFormat),
		EndDate:    b.EndDate().Format(dto.DateFormat),
		CreatedAt:  b.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}
}
