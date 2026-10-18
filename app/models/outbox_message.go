package models

import (
	"encoding/json"
	"time"
)

type OutboxMessage struct {
	ID        int64
	Payload   json.RawMessage
	Status    string
	Attempts  int
	CreatedAt time.Time
	SentAt    *time.Time
	LastError *string
}
