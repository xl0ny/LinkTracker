package domain

// Subscription represents a chat-link subscription used by the scheduler
// to iterate over all tracked links across chats with pagination.
type Subscription struct {
	ChatID int64
	Link   Link
}
