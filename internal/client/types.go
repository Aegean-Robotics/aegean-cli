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
