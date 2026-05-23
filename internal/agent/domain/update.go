package domain

import "time"

type RawUpdate struct {
	EventID     string
	OccurredAt  time.Time
	URL         string
	Description string
	Author      string
	TgChatIDs   []int64
}

type ProcessedUpdate struct {
	EventID     string
	OccurredAt  time.Time
	URL         string
	Description string
	TgChatIDs   []int64
	Priority    string
}
