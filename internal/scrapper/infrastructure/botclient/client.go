package botclient

import (
	"context"
	"fmt"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type client struct {
	apiClient *ClientWithResponses
}

//revive:disable-next-line:unexported-return returning *client is intentional (internal impl)
func NewNotifier(apiClient *ClientWithResponses) *client {
	return &client{apiClient: apiClient}
}

func (c *client) Notify(ctx context.Context, chatID int64, link domain.Link) error {
	chatIDs := []int64{chatID}
	req := LinkUpdate{
		Url:       &link.URL,
		TgChatIds: &chatIDs,
	}
	resp, err := c.apiClient.PostUpdatesWithResponse(ctx, req)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return fmt.Errorf("bot notify: status %d", resp.StatusCode())
	}
	return nil
}
