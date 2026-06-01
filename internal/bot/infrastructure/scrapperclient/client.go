package scrapperclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
)

type client struct {
	api *Client
}

func NewLinkTracker(serverURL string, httpClient *http.Client) (application.LinkTracker, error) {
	url := strings.TrimSpace(serverURL)
	if url == "" {
		return nil, errors.New("scrapper URL is required")
	}
	opts := []ClientOption{
		WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-Metrics-Source", "bot")
			return nil
		}),
	}
	if httpClient != nil {
		opts = append(opts, WithHTTPClient(httpClient))
	}
	api, err := NewClient(url, opts...)
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusConflict {
		return errors.New("link already exists")
	}
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("chat not found")
	}
	if resp.StatusCode >= http.StatusBadRequest {
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("link not found")
	}
	if resp.StatusCode >= http.StatusBadRequest {
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("chat not found")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("list links: status %d", resp.StatusCode)
	}
	var body struct {
		Links *[]struct {
			URL  *string   `json:"url"`
			Tags *[]string `json:"tags"`
		} `json:"links"`
	}
	if errDecode := json.NewDecoder(resp.Body).Decode(&body); errDecode != nil {
		return nil, fmt.Errorf("decode response: %w", errDecode)
	}
	if body.Links == nil {
		return nil, nil
	}
	out := make([]application.LinkInfo, 0, len(*body.Links))
	for _, l := range *body.Links {
		url := ""
		if l.URL != nil {
			url = *l.URL
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
