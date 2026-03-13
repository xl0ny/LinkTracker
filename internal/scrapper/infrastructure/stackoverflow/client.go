package stackoverflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.stackexchange.com/2.3"

type Client struct {
	http *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{http: httpClient}
}

type questionsResponse struct {
	Items []struct {
		LastActivityDate int64 `json:"last_activity_date"`
	} `json:"items"`
}

func (c *Client) CheckUpdated(ctx context.Context, questionURL string, prev time.Time) (changed bool, latest time.Time, err error) {
	id, err := parseQuestionID(questionURL)
	if err != nil {
		return false, time.Time{}, err
	}
	apiURL := baseURL + "/questions/" + strconv.FormatInt(id, 10) + "?site=stackoverflow"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, time.Time{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return false, time.Time{}, fmt.Errorf("stackoverflow api: status=%d", resp.StatusCode)
	}
	var body questionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, time.Time{}, err
	}
	if len(body.Items) == 0 {
		return false, time.Time{}, fmt.Errorf("question not found")
	}
	latest = time.Unix(body.Items[0].LastActivityDate, 0)
	if latest.After(prev) {
		return true, latest, nil
	}
	return false, latest, nil
}

func parseQuestionID(raw string) (int64, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "stackoverflow.com" && !strings.HasSuffix(u.Host, ".stackoverflow.com") {
		return 0, fmt.Errorf("not stackoverflow url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "questions" {
		return 0, fmt.Errorf("invalid path")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid question id")
	}
	return id, nil
}
