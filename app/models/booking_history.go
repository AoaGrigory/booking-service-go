package models

import "time"

type BookingHistory struct {
	id             int64
	bookingID      int64
	previousStatus BookingStatus
	newStatus      BookingStatus
	changedAt      time.Time
	reason         string
	initiator      string
}

func (b *BookingHistory) GetId() int64                  { return b.id }
func (b *BookingHistory) BookingID() int64              { return b.bookingID }
func (b *BookingHistory) PreviousStatus() BookingStatus { return b.previousStatus }
func (b *BookingHistory) NewStatus() BookingStatus      { return b.newStatus }
func (b *BookingHistory) ChangedAt() time.Time          { return b.changedAt }
func (b *BookingHistory) Reason() string                { return b.reason }
func (b *BookingHistory) Initiator() string             { return b.initiator }

func NewBookingHistory(bookingID int64, previousStatus, newStatus BookingStatus, reason, initiator string) (*BookingHistory, error) {
	if bookingID <= 0 {
		return nil, ErrInvalidBookingID
	}
	if !newStatus.IsValid() {
		return nil, ErrInvalidStatusTransition
	}

	return &BookingHistory{
		bookingID:      bookingID,
		previousStatus: previousStatus,
		newStatus:      newStatus,
		changedAt:      time.Now(),
		reason:         reason,
		initiator:      initiator,
	}, nil
}
func RestoreBookingHistory(
	id, bookingId int64,
	previousStatus, newStatus BookingStatus,
	changedAt time.Time,
	reason, initiator string) *BookingHistory {
	return &BookingHistory{
		id:             id,
		bookingID:      bookingId,
		previousStatus: previousStatus,
		newStatus:      newStatus,
		changedAt:      changedAt,
		reason:         reason,
		initiator:      initiator,
	}
}
