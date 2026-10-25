package notification

type SendNotificationRequest struct {
	BookingID int64  `json:"bookingId"`
	UserID    int64  `json:"userId"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
