package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ─── Auth ────────────────────────────────────────────────────────────────────

func (c *Client) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.Do(ctx, http.MethodPost, "/auth/login", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── API keys ────────────────────────────────────────────────────────────────

func (c *Client) CreateAPIKey(ctx context.Context, req CreateKeyRequest) (*CreateKeyResponse, error) {
	var out CreateKeyResponse
	if err := c.Do(ctx, http.MethodPost, "/api/keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKeyInfo, error) {
	var out []APIKeyInfo
	if err := c.Do(ctx, http.MethodGet, "/api/keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/api/keys/"+url.PathEscape(id), nil, nil)
}

// ─── Domains ─────────────────────────────────────────────────────────────────

func (c *Client) AddDomain(ctx context.Context, req AddDomainRequest) (*DomainInfo, error) {
	if req.Type == "" {
		req.Type = "CUSTOM"
	}
	var out DomainInfo
	if err := c.Do(ctx, http.MethodPost, "/api/domains", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListDomains(ctx context.Context) ([]DomainInfo, error) {
	var out []DomainInfo
	if err := c.Do(ctx, http.MethodGet, "/api/domains", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) VerifyDomain(ctx context.Context, id string) (*DomainInfo, error) {
	var out DomainInfo
	if err := c.Do(ctx, http.MethodPost, "/api/domains/"+url.PathEscape(id)+"/verify", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DomainChecks(ctx context.Context, id string) ([]DNSRecord, error) {
	var out []DNSRecord
	if err := c.Do(ctx, http.MethodGet, "/api/domains/"+url.PathEscape(id)+"/checks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindDomainByName scans the user's list for a matching domain name.
// Helpful for `aegean domains verify acme.com` (name) vs UUID.
func (c *Client) FindDomainByName(ctx context.Context, name string) (*DomainInfo, error) {
	all, err := c.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].DomainName == name {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("no domain named %q on this account (run `aegean domains list`)", name)
}

// ─── Email ───────────────────────────────────────────────────────────────────

func (c *Client) SendEmail(ctx context.Context, req SendEmailRequest) (*SendEmailResponse, error) {
	var out SendEmailResponse
	if err := c.Do(ctx, http.MethodPost, "/v1/email/send", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Logs ────────────────────────────────────────────────────────────────────

func (c *Client) ListLogs(ctx context.Context, page, size int) (*LogsPage, error) {
	path := fmt.Sprintf("/v1/logs?page=%d&size=%d", page, size)
	var out LogsPage
	if err := c.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
