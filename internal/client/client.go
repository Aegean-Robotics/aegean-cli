// Package client is the HTTP layer the CLI uses to talk to the Aegean backend.
//
// Two auth modes:
//
//	JWT     — sent as Authorization: Bearer <token>. Used for /auth, /api/keys, /api/domains.
//	API key — sent as X-API-Key: <key>.            Used for /v1/email, /v1/logs.
//
// Each Do* helper is small, typed, and returns a structured ErrAPI on non-2xx
// so commands can render useful messages instead of stack traces.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// Client wraps an http.Client + base URL + auth headers.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	UserAgent  string
	Token      string // JWT (optional)
	APIKey     string // X-API-Key (optional)
}

// New builds a Client with sane defaults.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: "aegean-cli",
	}
}

// WithToken returns a copy that sends the JWT on every request.
func (c *Client) WithToken(token string) *Client {
	out := *c
	out.Token = token
	return &out
}

// WithAPIKey returns a copy that sends the API key on every request.
func (c *Client) WithAPIKey(key string) *Client {
	out := *c
	out.APIKey = key
	return &out
}

// ErrAPI is returned when the server replies with a non-2xx status.
type ErrAPI struct {
	Status int
	Method string
	Path   string
	Body   string
}

func (e *ErrAPI) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
	}
	if len(body) > 400 {
		body = body[:400] + "…"
	}
	return fmt.Sprintf("%s %s: HTTP %d — %s", e.Method, e.Path, e.Status, body)
}

// Do is the low-level entry point. Marshals reqBody as JSON, decodes the
// response into out (which may be nil).
func (c *Client) Do(ctx context.Context, method, path string, reqBody, out any) error {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal %s %s body: %w", method, path, err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read %s %s response: %w", method, path, readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ErrAPI{Status: resp.StatusCode, Method: method, Path: path, Body: string(respBytes)}
	}

	if out == nil || resp.StatusCode == http.StatusNoContent || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("decode %s %s response: %w (body: %s)", method, path, err, truncate(string(respBytes), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// DoMultipart streams a multipart/form-data POST. Used today by
// `aegean sites deploy` to ship a bundle zip; future uploads (CLI
// attachments, templates) can reuse it. fileName is the form name of
// the file part ("file" for /api/sites/{id}/deployments); fileBytes
// is the entire payload, contentType the file's MIME, fields any
// additional plain-text form fields (e.g. notes=…).
//
// We pull the whole payload in memory before dispatch — fine because
// the backend caps bundles at 250 MB and the CLI doesn't have any
// other multipart use case yet. If we ever ship 1 GB uploads we'll
// switch to io.Pipe streaming and a longer client timeout.
func (c *Client) DoMultipart(ctx context.Context, path, fileFieldName, fileName string,
	fileBytes []byte, fileContentType string, fields map[string]string, out any) error {

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return fmt.Errorf("write field %q: %w", k, err)
		}
	}

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name=%q; filename=%q`, fileFieldName, fileName))
	if fileContentType != "" {
		hdr.Set("Content-Type", fileContentType)
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return fmt.Errorf("write file part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, &buf)
	if err != nil {
		return fmt.Errorf("build POST %s: %w", path, err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	// Bundle uploads can take a minute on a slow link or a fresh
	// 250 MB zip — temporarily bump the timeout on a per-call basis
	// rather than mutating the shared client.
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline || time.Until(deadline) < 2*time.Minute {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read POST %s response: %w", path, readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ErrAPI{Status: resp.StatusCode, Method: http.MethodPost, Path: path, Body: string(respBytes)}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("decode POST %s response: %w (body: %s)", path, err, truncate(string(respBytes), 200))
	}
	return nil
}
