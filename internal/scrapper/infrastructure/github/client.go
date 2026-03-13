package github

import (
	"context"
	"encoding/json"
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

func (c *Client) CheckUpdated(ctx context.Context, repoURL string, prev time.Time) (changed bool, latest time.Time, err error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return false, time.Time{}, err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, time.Time{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, time.Time{}, fmt.Errorf("repo not found: %s", repoURL)
	}
	if resp.StatusCode >= 300 {
		return false, time.Time{}, fmt.Errorf("github api error: status=%d", resp.StatusCode)
	}

	var body repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, time.Time{}, err
	}
	if body.UpdatedAt.After(prev) {
		return true, body.UpdatedAt, nil
	}
	return false, body.UpdatedAt, nil
}

func parseRepoURL(raw string) (owner, repo string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "github.com" {
		return "", "", fmt.Errorf("not a github url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid repo path: %s", u.Path)
	}
	return parts[0], parts[1], nil
}
