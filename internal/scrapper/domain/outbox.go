package domain

import "time"

type OutboxStatus string

const (
	OutboxStatusPending OutboxStatus = "pending"
	OutboxStatusSent    OutboxStatus = "sent"
	OutboxStatusFailed  OutboxStatus = "failed"
)

type OutboxEvent struct {
	ID          int64
	EventID     string
	Topic       string
	Key         []byte
	Payload     []byte
	Status      OutboxStatus
	Attempts    int
	LastError   string
	AvailableAt time.Time
	CreatedAt   time.Time
	SentAt      *time.Time
}
