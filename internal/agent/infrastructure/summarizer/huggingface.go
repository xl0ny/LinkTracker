package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultHFTimeout = 15 * time.Second

type HuggingFaceConfig struct {
	APIURL    string
	Token     string
	MaxLength int
	MinLength int
	Timeout   time.Duration
}

type HuggingFace struct {
	apiURL string
	token  string
	maxLen int
	minLen int
	client *http.Client
}

func NewHuggingFace(cfg HuggingFaceConfig) (*HuggingFace, error) {
	if strings.TrimSpace(cfg.APIURL) == "" {
		return nil, errors.New("huggingface-summarizer: api_url required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultHFTimeout
	}
	return &HuggingFace{
		apiURL: cfg.APIURL,
		token:  cfg.Token,
		maxLen: cfg.MaxLength,
		minLen: cfg.MinLength,
		client: &http.Client{Timeout: timeout},
	}, nil
}

type hfRequest struct {
	Inputs     string         `json:"inputs"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

type hfItem struct {
	SummaryText string `json:"summary_text"`
}

type hfErrorResponse struct {
	Error         string  `json:"error"`
	EstimatedTime float64 `json:"estimated_time"`
}

func (h *HuggingFace) Summarize(ctx context.Context, text string) (string, error) {
	body := hfRequest{
		Inputs:  text,
		Options: map[string]any{"wait_for_model": true},
	}
	params := map[string]any{}
	if h.maxLen > 0 {
		params["max_length"] = h.maxLen
	}
	if h.minLen > 0 {
		params["min_length"] = h.minLen
	}
	if len(params) > 0 {
		body.Parameters = params
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("huggingface-summarizer: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("huggingface-summarizer: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("huggingface-summarizer: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("huggingface-summarizer: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var hfErr hfErrorResponse
		_ = json.Unmarshal(respBody, &hfErr)
		if hfErr.Error != "" {
			return "", fmt.Errorf("huggingface-summarizer: status %d: %s", resp.StatusCode, hfErr.Error)
		}
		return "", fmt.Errorf("huggingface-summarizer: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var items []hfItem
	if jErr := json.Unmarshal(respBody, &items); jErr != nil {
		return "", fmt.Errorf("huggingface-summarizer: decode response: %w", jErr)
	}
	if len(items) == 0 || strings.TrimSpace(items[0].SummaryText) == "" {
		return "", errors.New("huggingface-summarizer: empty summary")
	}
	return items[0].SummaryText, nil
}
