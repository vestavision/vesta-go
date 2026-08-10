package vesta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.vestavision.io"

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(base string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("vesta: %s (%s)", e.Message, e.Code)
	}
	return fmt.Sprintf("vesta: %s", e.Message)
}

type Model struct {
	ModelID              string   `json:"model_id"`
	Title                string   `json:"title"`
	Category             string   `json:"category"`
	Description          string   `json:"description"`
	SupportsSync         bool     `json:"supports_sync"`
	SupportsAsync        bool     `json:"supports_async"`
	CreditCost           int      `json:"credit_cost"`
	RequiredCapabilities []string `json:"required_capabilities"`
}

type SubmitOptions struct {
	Input          map[string]any
	WebhookURL     string
	Sync           bool
	IdempotencyKey string
}

type Request struct {
	RequestID       string         `json:"request_id"`
	ModelID         string         `json:"model_id"`
	Status          string         `json:"status"`
	Mode            string         `json:"mode"`
	Output          map[string]any `json:"output,omitempty"`
	Error           map[string]any `json:"error,omitempty"`
	CreditsReserved int            `json:"credits_reserved,omitempty"`
}

type Credits struct {
	OrganizationID string `json:"organization_id"`
	Balance        int    `json:"balance"`
}

func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	var out struct {
		Models []Model `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/models", nil, "", &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func (c *Client) GetModel(ctx context.Context, modelID string) (*Model, error) {
	var out Model
	if err := c.do(ctx, http.MethodGet, "/api/v1/models/"+escapeModelID(modelID), nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Submit(ctx context.Context, modelID string, opts SubmitOptions) (*Request, error) {
	body := map[string]any{
		"input":       opts.Input,
		"webhook_url": opts.WebhookURL,
		"sync":        opts.Sync,
	}
	var out Request
	if err := c.do(ctx, http.MethodPost, "/api/v1/models/"+escapeModelID(modelID), body, opts.IdempotencyKey, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRequest(ctx context.Context, requestID string) (*Request, error) {
	var out Request
	if err := c.do(ctx, http.MethodGet, "/api/v1/requests/"+url.PathEscape(requestID), nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Cancel(ctx context.Context, requestID string) (*Request, error) {
	var out Request
	if err := c.do(ctx, http.MethodDelete, "/api/v1/requests/"+url.PathEscape(requestID), nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Credits(ctx context.Context) (*Credits, error) {
	var out Credits
	if err := c.do(ctx, http.MethodGet, "/api/v1/credits", nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PresignUpload(ctx context.Context, contentType string) (map[string]any, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/v1/storage/upload-url", map[string]any{
		"content_type": contentType,
	}, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Wait polls GetRequest until a terminal status or timeout.
func (c *Client) Wait(ctx context.Context, requestID string, interval, timeout time.Duration) (*Request, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		req, err := c.GetRequest(ctx, requestID)
		if err != nil {
			return nil, err
		}
		switch req.Status {
		case "completed", "failed", "canceled":
			return req, nil
		}
		if time.Now().After(deadline) {
			return req, fmt.Errorf("vesta: wait timeout for %s (status=%s)", requestID, req.Status)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func escapeModelID(modelID string) string {
	// model ids are like vesta/face-landmarks — keep slash for chi wildcard routes
	parts := strings.Split(modelID, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (c *Client) do(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	if c.apiKey == "" {
		return &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, raw)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func parseAPIError(status int, raw []byte) error {
	var wrap struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && (wrap.Error.Code != "" || wrap.Error.Message != "") {
		msg := wrap.Error.Message
		if msg == "" {
			msg = wrap.Error.Code
		}
		return &APIError{Status: status, Code: wrap.Error.Code, Message: msg}
	}
	var legacy struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.Error != "" {
		return &APIError{Status: status, Code: legacy.Error, Message: legacy.Error}
	}
	return &APIError{Status: status, Code: "http_error", Message: string(raw)}
}
