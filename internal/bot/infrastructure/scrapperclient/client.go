package scrapperclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
)

type client struct {
	api *Client
}

func NewLinkTracker(serverURL string) (application.LinkTracker, error) {
	url := strings.TrimSpace(serverURL)
	if url == "" {
		return nil, fmt.Errorf("scrapper URL is required")
	}
	api, err := NewClient(url)
	if err != nil {
		return nil, err
	}
	return &client{api: api}, nil
}

func (c *client) RegisterChat(ctx context.Context, chatID int64) error {
	resp, err := c.api.PostTgChatId(ctx, chatID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("register chat: status %d", resp.StatusCode)
	}
	return nil
}

func (c *client) AddLink(ctx context.Context, chatID int64, link string, tags []string) error {
	req := PostLinksJSONRequestBody{
		Link: &link,
		Tags: &tags,
	}
	params := &PostLinksParams{TgChatId: chatID}
	resp, err := c.api.PostLinks(ctx, params, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("link already exists")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("chat not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("add link: status %d", resp.StatusCode)
	}
	return nil
}

func (c *client) RemoveLink(ctx context.Context, chatID int64, link string) error {
	req := RemoveLinkRequest{Link: &link}
	params := &DeleteLinksParams{TgChatId: chatID}
	resp, err := c.api.DeleteLinks(ctx, params, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("link not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remove link: status %d", resp.StatusCode)
	}
	return nil
}

func (c *client) ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]application.LinkInfo, error) {
	params := &GetLinksParams{TgChatId: chatID}
	resp, err := c.api.GetLinks(ctx, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("chat not found")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list links: status %d", resp.StatusCode)
	}
	var body struct {
		Links *[]struct {
			Url  *string  `json:"url"`
			Tags *[]string `json:"tags"`
		} `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Links == nil {
		return nil, nil
	}
	out := make([]application.LinkInfo, 0, len(*body.Links))
	for _, l := range *body.Links {
		url := ""
		if l.Url != nil {
			url = *l.Url
		}
		tags := []string{}
		if l.Tags != nil {
			tags = *l.Tags
		}
		if tagFilter != "" {
			found := false
			for _, t := range tags {
				if t == tagFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, application.LinkInfo{URL: url, Tags: tags})
	}
	return out, nil
}
