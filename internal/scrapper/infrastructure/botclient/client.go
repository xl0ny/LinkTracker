package botclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type client struct {
	apiClient *ClientWithResponses
}

//revive:disable-next-line:unexported-return returning *client is intentional (internal impl)
func NewNotifier(apiClient *ClientWithResponses) *client {
	return &client{apiClient: apiClient}
}

func (c *client) Notify(ctx context.Context, chatID int64, link domain.Link, description string) error {
	chatIDs := []int64{chatID}
	req := LinkUpdate{
		Url:       &link.URL,
		TgChatIds: &chatIDs,
	}
	if description != "" {
		req.Description = &description
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

func (c *client) NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error {
	if len(links) == 0 {
		return nil
	}
	chatIDs := []int64{chatID}
	description := "Не удалось обработать ссылки:\n" + strings.Join(links, "\n")
	req := LinkUpdate{
		Description: &description,
		TgChatIds:   &chatIDs,
	}
	resp, err := c.apiClient.PostUpdatesWithResponse(ctx, req)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return fmt.Errorf("bot notify failed links: status %d", resp.StatusCode())
	}
	return nil
}
