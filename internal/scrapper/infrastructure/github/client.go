package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	http    *http.Client
	token   string
	baseURL string
}

func NewClient(httpClient *http.Client, token string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		http:    httpClient,
		token:   token,
		baseURL: "https://api.github.com",
	}
}

type repoResponse struct {
	UpdatedAt time.Time `json:"updated_at"`
}

// CheckUpdated возвращает время последнего обновления репозитория по API; сравнение с ранее сохранённым временем — на уровне вызывающего кода.
func (c *Client) CheckUpdated(ctx context.Context, repoURL string) (latest time.Time, err error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return time.Time{}, err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return time.Time{}, fmt.Errorf("repo not found: %s", repoURL)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return time.Time{}, fmt.Errorf("github api error: status=%d", resp.StatusCode)
	}

	var body repoResponse
	if errDecode := json.NewDecoder(resp.Body).Decode(&body); errDecode != nil {
		return time.Time{}, fmt.Errorf("decode: %w", errDecode)
	}
	return body.UpdatedAt, nil
}

func parseRepoURL(raw string) (owner, repo string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "github.com" {
		return "", "", errors.New("not a github url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	const minPathParts = 2
	if len(parts) < minPathParts {
		return "", "", fmt.Errorf("invalid repo path: %s", u.Path)
	}
	return parts[0], parts[1], nil
}
