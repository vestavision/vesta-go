package vesta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vestavision/vesta-go/generated"
)

const DefaultBaseURL = "https://api.vestavision.io"

type Client struct {
	api    *generated.APIClient
	cfg    *generated.Configuration
	apiKey string
}

type Option func(*Client)

func WithBaseURL(base string) Option {
	return func(c *Client) {
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		if base == "" {
			return
		}
		c.cfg.Servers = generated.ServerConfigurations{{URL: base}}
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.cfg.HTTPClient = hc
		}
	}
}

func NewClient(apiKey string, opts ...Option) *Client {
	cfg := generated.NewConfiguration()
	cfg.Servers = generated.ServerConfigurations{{URL: DefaultBaseURL}}
	cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	c := &Client{cfg: cfg, apiKey: strings.TrimSpace(apiKey)}
	for _, opt := range opts {
		opt(c)
	}
	c.api = generated.NewAPIClient(c.cfg)
	return c
}

func (c *Client) auth(ctx context.Context) context.Context {
	return context.WithValue(ctx, generated.ContextAPIKeys, map[string]generated.APIKey{
		"ApiKeyAuth": {Prefix: "Key", Key: c.apiKey},
	})
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
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	list, _, err := c.api.ModelsAPI.ListModels(c.auth(ctx)).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := make([]Model, 0, len(list.Models))
	for _, m := range list.Models {
		out = append(out, modelFromGenerated(m))
	}
	return out, nil
}

func (c *Client) GetModel(ctx context.Context, modelID string) (*Model, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	m, _, err := c.api.ModelsAPI.GetModel(c.auth(ctx), modelID).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := modelFromGenerated(*m)
	return &out, nil
}

func (c *Client) Submit(ctx context.Context, modelID string, opts SubmitOptions) (*Request, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	body := generated.NewSubmitBody(opts.Input)
	if opts.WebhookURL != "" {
		body.SetWebhookUrl(opts.WebhookURL)
	}
	body.SetSync(opts.Sync)
	req := c.api.ModelsAPI.SubmitModel(c.auth(ctx), modelID).SubmitBody(*body)
	if opts.IdempotencyKey != "" {
		req = req.IdempotencyKey(opts.IdempotencyKey)
	}
	raw, _, err := req.Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := requestFromGenerated(raw)
	return &out, nil
}

func (c *Client) GetRequest(ctx context.Context, requestID string) (*Request, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	raw, _, err := c.api.RequestsAPI.GetRequest(c.auth(ctx), requestID).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := requestFromGenerated(raw)
	return &out, nil
}

// View returns the current job snapshot (status, output, error) without waiting.
func (c *Client) View(ctx context.Context, requestID string) (*Request, error) {
	return c.GetRequest(ctx, requestID)
}

func (c *Client) ListRequests(ctx context.Context) ([]Request, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	list, _, err := c.api.RequestsAPI.ListRequests(c.auth(ctx)).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := make([]Request, 0, len(list.Requests))
	for i := range list.Requests {
		out = append(out, requestFromGenerated(&list.Requests[i]))
	}
	return out, nil
}

func (c *Client) Cancel(ctx context.Context, requestID string) (*Request, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	raw, _, err := c.api.RequestsAPI.CancelRequest(c.auth(ctx), requestID).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := requestFromGenerated(raw)
	return &out, nil
}

func (c *Client) Credits(ctx context.Context) (*Credits, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	raw, _, err := c.api.CreditsAPI.GetCredits(c.auth(ctx)).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	out := Credits{}
	if raw.OrganizationId != nil {
		out.OrganizationID = *raw.OrganizationId
	}
	if raw.Balance != nil {
		out.Balance = int(*raw.Balance)
	}
	return &out, nil
}

func (c *Client) PresignUpload(ctx context.Context, contentType string) (map[string]any, error) {
	if c.apiKey == "" {
		return nil, &APIError{Status: 401, Code: "unauthorized", Message: "api key required"}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	body := generated.NewPresignUploadBody()
	body.SetContentType(contentType)
	raw, _, err := c.api.StorageAPI.PresignUpload(c.auth(ctx)).PresignUploadBody(*body).Execute()
	if err != nil {
		return nil, mapAPIError(err)
	}
	return map[string]any{
		"key":        raw.Key,
		"upload_url": raw.UploadUrl,
		"public_url": raw.PublicUrl,
	}, nil
}

// Wait polls View until a terminal status or timeout.
func (c *Client) Wait(ctx context.Context, requestID string, interval, timeout time.Duration) (*Request, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		req, err := c.View(ctx, requestID)
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

func modelFromGenerated(m generated.Model) Model {
	out := Model{
		RequiredCapabilities: m.RequiredCapabilities,
	}
	if m.ModelId != nil {
		out.ModelID = *m.ModelId
	}
	if m.Title != nil {
		out.Title = *m.Title
	}
	if m.Category != nil {
		out.Category = *m.Category
	}
	if m.Description != nil {
		out.Description = *m.Description
	}
	if m.SupportsSync != nil {
		out.SupportsSync = *m.SupportsSync
	}
	if m.SupportsAsync != nil {
		out.SupportsAsync = *m.SupportsAsync
	}
	if m.CreditCost != nil {
		out.CreditCost = int(*m.CreditCost)
	}
	return out
}

func requestFromGenerated(r *generated.Request) Request {
	if r == nil {
		return Request{}
	}
	out := Request{
		Output: r.Output,
	}
	if r.RequestId != nil {
		out.RequestID = *r.RequestId
	}
	if r.ModelId != nil {
		out.ModelID = *r.ModelId
	}
	if r.Status != nil {
		out.Status = *r.Status
	}
	if r.Mode != nil {
		out.Mode = *r.Mode
	}
	if r.CreditsReserved != nil {
		out.CreditsReserved = int(*r.CreditsReserved)
	}
	if r.Error != nil {
		out.Error = map[string]any{"code": r.Error.Code, "message": r.Error.Message}
	}
	return out
}

func mapAPIError(err error) error {
	if err == nil {
		return nil
	}
	openErr, ok := err.(*generated.GenericOpenAPIError)
	if !ok {
		return err
	}
	status := 0
	msg := openErr.Error()
	code := "http_error"
	if body := openErr.Body(); len(body) > 0 {
		var wrap struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &wrap) == nil && (wrap.Error.Code != "" || wrap.Error.Message != "") {
			code = wrap.Error.Code
			msg = wrap.Error.Message
			if msg == "" {
				msg = code
			}
		}
	}
	if model, ok := openErr.Model().(generated.ErrorBody); ok && model.Error.Code != "" {
		code = model.Error.Code
		msg = model.Error.Message
	}
	// Status is embedded in the error string like "401 Unauthorized" for some paths.
	if len(msg) >= 3 {
		var n int
		if _, scanErr := fmt.Sscanf(openErr.Error(), "%d", &n); scanErr == nil {
			status = n
		}
	}
	return &APIError{Status: status, Code: code, Message: msg}
}
