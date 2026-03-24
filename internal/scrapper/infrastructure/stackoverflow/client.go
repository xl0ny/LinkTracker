package stackoverflow

import (
	"context"
	"encoding/json"
	"errors"
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

// CheckUpdated возвращает время последней активности вопроса по API; сравнение с ранее сохранённым временем — на уровне вызывающего кода.
func (c *Client) CheckUpdated(ctx context.Context, questionURL string) (latest time.Time, err error) {
	id, err := parseQuestionID(questionURL)
	if err != nil {
		return time.Time{}, err
	}
	apiURL := baseURL + "/questions/" + strconv.FormatInt(id, 10) + "?site=stackoverflow"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return time.Time{}, fmt.Errorf("stackoverflow api: status=%d", resp.StatusCode)
	}
	var body questionsResponse
	if errDecode := json.NewDecoder(resp.Body).Decode(&body); errDecode != nil {
		return time.Time{}, fmt.Errorf("decode: %w", errDecode)
	}
	if len(body.Items) == 0 {
		return time.Time{}, errors.New("question not found")
	}
	return time.Unix(body.Items[0].LastActivityDate, 0), nil
}

func parseQuestionID(raw string) (int64, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "stackoverflow.com" && !strings.HasSuffix(u.Host, ".stackoverflow.com") {
		return 0, errors.New("not stackoverflow url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	const minPathParts = 2
	if len(parts) < minPathParts || parts[0] != "questions" {
		return 0, errors.New("invalid path")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, errors.New("invalid question id")
	}
	return id, nil
}
