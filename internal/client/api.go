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

// ─── Account ─────────────────────────────────────────────────────────────────

// CurrentAccount returns the calling user's account + membership in the
// account scoped by the JWT. Used by `aegean sites deploy` to print the
// real path URL post-deploy instead of a <your-alias> placeholder.
func (c *Client) CurrentAccount(ctx context.Context) (*AccountView, error) {
	var out AccountView
	if err := c.Do(ctx, http.MethodGet, "/v1/account", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Sites (Phase 1-4b) ──────────────────────────────────────────────────────

// ListSites returns every Site visible to the current account.
func (c *Client) ListSites(ctx context.Context) ([]Site, error) {
	var out []Site
	if err := c.Do(ctx, http.MethodGet, "/api/sites", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindSite resolves a slug → Site by listing + filtering client-side. The
// backend doesn't expose a /api/sites/by-slug/{slug} endpoint today; a list
// hop is cheap (one tenant rarely has more than a handful of sites).
func (c *Client) FindSite(ctx context.Context, slug string) (*Site, error) {
	all, err := c.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Slug == slug {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("no site with slug %q on this account (run `aegean sites list`)", slug)
}

// CreateSite POSTs a new Site row.
func (c *Client) CreateSite(ctx context.Context, in SiteInput) (*Site, error) {
	var out Site
	if err := c.Do(ctx, http.MethodPost, "/api/sites", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSite PUTs a full Site payload.
func (c *Client) UpdateSite(ctx context.Context, siteID string, in SiteInput) (*Site, error) {
	var out Site
	if err := c.Do(ctx, http.MethodPut, "/api/sites/"+url.PathEscape(siteID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSite removes the Site row. 204 on success.
func (c *Client) DeleteSite(ctx context.Context, siteID string) error {
	return c.Do(ctx, http.MethodDelete, "/api/sites/"+url.PathEscape(siteID), nil, nil)
}

// DeploySite uploads a zip bundle. The backend extracts + activates atomically.
// notes is optional (shows up in `aegean sites history`).
func (c *Client) DeploySite(ctx context.Context, siteID string, zipBytes []byte, zipName, notes string) (*SiteDeployment, error) {
	var out SiteDeployment
	fields := map[string]string{}
	if notes != "" {
		fields["notes"] = notes
	}
	if err := c.DoMultipart(ctx,
		"/api/sites/"+url.PathEscape(siteID)+"/deployments",
		"file", zipName, zipBytes, "application/zip", fields, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSiteDeployments returns the per-Site deployment ledger, newest first.
func (c *Client) ListSiteDeployments(ctx context.Context, siteID string) ([]SiteDeployment, error) {
	var out []SiteDeployment
	if err := c.Do(ctx, http.MethodGet,
		"/api/sites/"+url.PathEscape(siteID)+"/deployments", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ActivateDeployment flips the Site's active pointer at a prior deployment
// (rollback / roll-forward).
func (c *Client) ActivateDeployment(ctx context.Context, siteID, deploymentID string) error {
	return c.Do(ctx, http.MethodPost,
		"/api/sites/"+url.PathEscape(siteID)+"/activate/"+url.PathEscape(deploymentID),
		nil, nil)
}
