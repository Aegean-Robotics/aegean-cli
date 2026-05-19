package client

import "time"

// ─── Auth ────────────────────────────────────────────────────────────────────

type LoginRequest struct {
	AccountAlias string `json:"accountAlias,omitempty"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

type LoginResponse struct {
	Token     string  `json:"token"`
	UserID    string  `json:"userId"`
	AccountID string  `json:"accountId"`
	Email     string  `json:"email"`
	Name      string  `json:"name"`
	Balance   float64 `json:"balance"`
	PlanType  string  `json:"planType"`
	Role      string  `json:"role"`
}

// ─── API keys ────────────────────────────────────────────────────────────────

type CreateKeyRequest struct {
	Name      string `json:"name"`
	RateLimit *int   `json:"rateLimit,omitempty"`
}

type CreateKeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
}

type APIKeyInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"keyPrefix"`
	RateLimit  int        `json:"rateLimit"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// ─── Domains ─────────────────────────────────────────────────────────────────

type AddDomainRequest struct {
	DomainName string `json:"domainName"`
	Type       string `json:"type"` // CUSTOM | SUBDOMAIN
	Intent     string `json:"intent,omitempty"`
}

type DNSRecord struct {
	Type     string `json:"type"`
	Record   string `json:"record"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	Purpose  string `json:"purpose"`
	Required bool   `json:"required"`
	Verified *bool  `json:"verified,omitempty"`
}

type DomainInfo struct {
	ID            string      `json:"id"`
	DomainName    string      `json:"domainName"`
	Type          string      `json:"type"`
	Verified      bool        `json:"verified"`
	DKIMSelector  string      `json:"dkimSelector,omitempty"`
	DKIMPublicKey string      `json:"dkimPublicKey,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	DNSRecords    []DNSRecord `json:"dnsRecords,omitempty"`
}

// ─── Email send ──────────────────────────────────────────────────────────────

type SendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	From    string `json:"from,omitempty"`
	ReplyTo string `json:"replyTo,omitempty"`
}

type SendEmailResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ─── Logs ────────────────────────────────────────────────────────────────────

type EmailLog struct {
	ID         string     `json:"id"`
	Recipient  string     `json:"recipient"`
	Sender     string     `json:"sender,omitempty"`
	Subject    string     `json:"subject"`
	Status     string     `json:"status"`
	Error      string     `json:"error,omitempty"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
	OpenedAt   *time.Time `json:"openedAt,omitempty"`
	ClickedAt  *time.Time `json:"clickedAt,omitempty"`
	BouncedAt  *time.Time `json:"bouncedAt,omitempty"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
}

// LogsPage mirrors Spring's Page<T> wire shape (snake-case fields are not used
// here — Spring serialises as camelCase via Jackson defaults).
type LogsPage struct {
	Content       []EmailLog `json:"content"`
	TotalElements int64      `json:"totalElements"`
	TotalPages    int        `json:"totalPages"`
	Number        int        `json:"number"`
	Size          int        `json:"size"`
	First         bool       `json:"first"`
	Last          bool       `json:"last"`
	NumberOfElements int     `json:"numberOfElements"`
}

// ─── Account ─────────────────────────────────────────────────────────────────

// AccountView mirrors AccountController.AccountView. Carries the alias the
// dashboard + path URL surface key off — the CLI prints it after `aegean
// sites deploy` so the user sees the live URL rather than a placeholder.
type AccountView struct {
	AccountID    string  `json:"accountId"`
	Alias        string  `json:"alias"`
	Name         string  `json:"name"`
	PlanType     string  `json:"planType"`
	Balance      float64 `json:"balance"`
	MyRole       string  `json:"myRole"`
	MembershipID string  `json:"membershipId"`
}

// ─── Static sites (Phase 1-4b) ───────────────────────────────────────────────

// Site mirrors the backend's Site entity JSON shape.
type Site struct {
	ID                  string    `json:"id"`
	Slug                string    `json:"slug"`
	BucketName          string    `json:"bucketName"`
	KeyPrefix           string    `json:"keyPrefix"`
	IndexDocument       string    `json:"indexDocument"`
	ErrorDocument       string    `json:"errorDocument"`
	SpaFallback         bool      `json:"spaFallback"`
	DefaultCacheMaxAge  int       `json:"defaultCacheMaxAge"`
	Enabled             bool      `json:"enabled"`
	CustomDomain        string    `json:"customDomain,omitempty"`
	PreferredSubdomain  string    `json:"preferredSubdomain,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// SiteInput is the create/update payload — matches SiteService.SiteInput on
// the backend. Pointer fields stay omitempty-clean so an unset bool doesn't
// flip the persisted value to false.
type SiteInput struct {
	Slug                string `json:"slug"`
	BucketName          string `json:"bucketName"`
	KeyPrefix           string `json:"keyPrefix,omitempty"`
	IndexDocument       string `json:"indexDocument,omitempty"`
	ErrorDocument       string `json:"errorDocument,omitempty"`
	SpaFallback         *bool  `json:"spaFallback,omitempty"`
	DefaultCacheMaxAge  *int   `json:"defaultCacheMaxAge,omitempty"`
	Enabled             *bool  `json:"enabled,omitempty"`
	CustomDomain        string `json:"customDomain,omitempty"`
	PreferredSubdomain  string `json:"preferredSubdomain,omitempty"`
}

// SiteDeployment is the per-bundle ledger row — one per `aegean sites deploy`.
type SiteDeployment struct {
	ID               string    `json:"id"`
	DeploymentPrefix string    `json:"deploymentPrefix"`
	BytesTotal       int64     `json:"bytesTotal"`
	FileCount        int       `json:"fileCount"`
	Notes            string    `json:"notes,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}
