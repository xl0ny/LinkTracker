package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/linkedin/goavro/v2"
)

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu    sync.RWMutex
	cache map[int32]*goavro.Codec
}

func NewClient(baseURL string) *Client {
	u := strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL:    u,
		httpClient: http.DefaultClient,
		cache:      make(map[int32]*goavro.Codec),
	}
}

type registerResponse struct {
	ID int `json:"id"`
}

type schemaByIDResponse struct {
	Schema string `json:"schema"`
}

func (c *Client) RegisterSchema(ctx context.Context, subject, schemaJSON string) (int32, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)
	body, err := json.Marshal(map[string]string{"schema": schemaJSON})
	if err != nil {
		return 0, fmt.Errorf("schema registry: marshal register body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("schema registry: new register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("schema registry: do register request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	b, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, fmt.Errorf("schema registry: read register response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("schema registry register %s: %s: %s", subject, resp.Status, strings.TrimSpace(string(b)))
	}

	var out registerResponse
	if unmErr := json.Unmarshal(b, &out); unmErr != nil {
		return 0, fmt.Errorf("schema registry register decode: %w", unmErr)
	}
	return int32(out.ID), nil
}

func (c *Client) CodecForID(ctx context.Context, schemaID int32) (*goavro.Codec, error) {
	c.mu.RLock()
	if codec, ok := c.cache[schemaID]; ok {
		c.mu.RUnlock()
		return codec, nil
	}
	c.mu.RUnlock()

	url := fmt.Sprintf("%s/schemas/ids/%d", c.baseURL, schemaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("schema registry: new get-schema request id=%d: %w", schemaID, err)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("schema registry: do get-schema request id=%d: %w", schemaID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("schema registry: read schema response id=%d: %w", schemaID, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("schema registry get schema id=%d: %s: %s", schemaID, resp.Status, strings.TrimSpace(string(b)))
	}

	var out schemaByIDResponse
	if unmErr := json.Unmarshal(b, &out); unmErr != nil {
		return nil, fmt.Errorf("schema registry get schema decode id=%d: %w", schemaID, unmErr)
	}
	codec, err := goavro.NewCodec(out.Schema)
	if err != nil {
		return nil, fmt.Errorf("goavro new codec id=%d: %w", schemaID, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[schemaID] = codec
	return codec, nil
}

type Encoder struct {
	client      *Client
	updateCodec *goavro.Codec
	failedCodec *goavro.Codec
	updateID    int32
	failedID    int32
}

func NewEncoder(ctx context.Context, baseURL, updateSubject, failedSubject, updateSchemaPath, failedSchemaPath string) (*Encoder, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("schema registry: empty base URL")
	}
	updateSchema, err := os.ReadFile(updateSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema registry: read update schema: %w", err)
	}
	failedSchema, err := os.ReadFile(failedSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema registry: read failed schema: %w", err)
	}
	client := NewClient(baseURL)
	updateCodec, err := goavro.NewCodec(string(updateSchema))
	if err != nil {
		return nil, fmt.Errorf("update codec: %w", err)
	}
	failedCodec, err := goavro.NewCodec(string(failedSchema))
	if err != nil {
		return nil, fmt.Errorf("failed codec: %w", err)
	}
	ui, err := client.RegisterSchema(ctx, updateSubject, string(updateSchema))
	if err != nil {
		return nil, fmt.Errorf("register update subject: %w", err)
	}
	fi, err := client.RegisterSchema(ctx, failedSubject, string(failedSchema))
	if err != nil {
		return nil, fmt.Errorf("register failed subject: %w", err)
	}
	return &Encoder{
		client:      client,
		updateCodec: updateCodec,
		failedCodec: failedCodec,
		updateID:    ui,
		failedID:    fi,
	}, nil
}

func (e *Encoder) EncodeUpdate(native map[string]any) ([]byte, error) {
	datum, err := e.updateCodec.BinaryFromNative(nil, native)
	if err != nil {
		return nil, fmt.Errorf("encode update native: %w", err)
	}
	return EncodeConfluent(e.updateID, datum), nil
}

func (e *Encoder) EncodeFailed(native map[string]any) ([]byte, error) {
	datum, err := e.failedCodec.BinaryFromNative(nil, native)
	if err != nil {
		return nil, fmt.Errorf("encode failed native: %w", err)
	}
	return EncodeConfluent(e.failedID, datum), nil
}
