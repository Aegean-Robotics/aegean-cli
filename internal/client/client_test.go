package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestLoginSendsExpectedBody(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q", r.Method)
		}
		var got LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got.Email != "ada@example.com" || got.Password != "hunter2" || got.AccountAlias != "acme" {
			t.Errorf("body: %+v", got)
		}
		_ = json.NewEncoder(w).Encode(LoginResponse{Token: "jwt-1", Email: got.Email})
	})

	c := New(srv.URL)
	out, err := c.Login(context.Background(), LoginRequest{AccountAlias: "acme", Email: "ada@example.com", Password: "hunter2"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if out.Token != "jwt-1" {
		t.Errorf("token: got %q", out.Token)
	}
}

func TestCreateAPIKeySendsBearerToken(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-abc" {
			t.Errorf("Authorization header: got %q", got)
		}
		_ = json.NewEncoder(w).Encode(CreateKeyResponse{ID: "k1", Name: "ci", Key: "aegean_sk_x.y"})
	})

	c := New(srv.URL).WithToken("jwt-abc")
	out, err := c.CreateAPIKey(context.Background(), CreateKeyRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if out.Key != "aegean_sk_x.y" {
		t.Errorf("key: got %q", out.Key)
	}
}

func TestSendEmailSendsAPIKeyHeader(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "aegean_sk_x.y" {
			t.Errorf("X-API-Key header: got %q", got)
		}
		_ = json.NewEncoder(w).Encode(SendEmailResponse{ID: "log1", Status: "sent"})
	})

	c := New(srv.URL).WithAPIKey("aegean_sk_x.y")
	out, err := c.SendEmail(context.Background(), SendEmailRequest{To: "a@b.com", Subject: "s", HTML: "<p>hi</p>"})
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if out.Status != "sent" {
		t.Errorf("status: got %q", out.Status)
	}
}

func TestNon2xxReturnsErrAPI(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
	})
	c := New(srv.URL)
	_, err := c.Login(context.Background(), LoginRequest{Email: "x@y", Password: "z"})
	if err == nil {
		t.Fatalf("want error")
	}
	var apiErr *ErrAPI
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *ErrAPI, got %T", err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Errorf("status: got %d", apiErr.Status)
	}
	if !strings.Contains(apiErr.Body, "invalid credentials") {
		t.Errorf("body: got %q", apiErr.Body)
	}
}

func TestFindDomainByName(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]DomainInfo{
			{ID: "1", DomainName: "foo.com"},
			{ID: "2", DomainName: "acme.com"},
		})
	})
	c := New(srv.URL).WithToken("jwt")
	got, err := c.FindDomainByName(context.Background(), "acme.com")
	if err != nil || got.ID != "2" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	_, err = c.FindDomainByName(context.Background(), "missing.com")
	if err == nil || !strings.Contains(err.Error(), "no domain named") {
		t.Errorf("want not-found error, got %v", err)
	}
}
