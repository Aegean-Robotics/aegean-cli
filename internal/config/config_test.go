package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tempPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		Dir:         dir,
		Config:      filepath.Join(dir, configFileName),
		Credentials: filepath.Join(dir, credentialsName),
	}
}

func TestResolveEndpoint(t *testing.T) {
	cases := []struct {
		in, want string
		err      bool
	}{
		{"", "https://api.aegeanengine.com", false},
		{"prod", "https://api.aegeanengine.com", false},
		{"dev", "https://api.dev.aegeanengine.com", false},
		{"local", "http://localhost:3022", false},
		{"https://custom.example.com/", "https://custom.example.com", false},
		{"http://localhost:9999", "http://localhost:9999", false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ResolveEndpoint(c.in)
		if c.err && err == nil {
			t.Errorf("ResolveEndpoint(%q): want error, got %q", c.in, got)
		}
		if !c.err && got != c.want {
			t.Errorf("ResolveEndpoint(%q) = %q, want %q (err=%v)", c.in, got, c.want, err)
		}
	}
}

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	paths := tempPaths(t)
	want := Config{
		Endpoint:       "dev",
		DefaultProfile: "alice",
		Output:         "json",
		Telemetry:      true,
	}
	if err := SaveConfig(paths, want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got != want {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSaveAndLoadCredentialsRoundTrip(t *testing.T) {
	paths := tempPaths(t)
	want := Credentials{Profiles: map[string]Profile{
		"default": {Token: "jwt-abc", Email: "user@example.com", AccountAlias: "acme"},
		"work":    {Token: "jwt-xyz", APIKey: "aegean_sk_xxx.yyy"},
	}}
	if err := SaveCredentials(paths, want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	got, err := LoadCredentials(paths)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if len(got.Profiles) != 2 || got.Profiles["default"].Token != "jwt-abc" || got.Profiles["work"].APIKey != "aegean_sk_xxx.yyy" {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestCredentialsRefuseInsecurePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms only")
	}
	paths := tempPaths(t)
	if err := os.WriteFile(paths.Credentials, []byte("[profiles]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadCredentials(paths)
	if err == nil || !strings.Contains(err.Error(), "insecure permissions") {
		t.Fatalf("want insecure-perm refusal, got %v", err)
	}
}

func TestResolveLayering(t *testing.T) {
	paths := tempPaths(t)
	_ = SaveConfig(paths, Config{Endpoint: "dev", Output: "yaml"})
	_ = SaveCredentials(paths, Credentials{Profiles: map[string]Profile{
		"default": {Token: "jwt-from-file", APIKey: "key-from-file"},
	}})

	t.Setenv("AEGEAN_ENDPOINT", "")
	t.Setenv("AEGEAN_PROFILE", "")
	t.Setenv("AEGEAN_OUTPUT", "")
	t.Setenv("AEGEAN_TOKEN", "")
	t.Setenv("AEGEAN_API_KEY", "")

	r, err := Resolve(paths, "", "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Endpoint != "https://api.dev.aegeanengine.com" {
		t.Errorf("endpoint from file: got %q", r.Endpoint)
	}
	if r.Output != "yaml" {
		t.Errorf("output from file: got %q", r.Output)
	}
	if r.Token != "jwt-from-file" || r.APIKey != "key-from-file" {
		t.Errorf("creds from file: token=%q apiKey=%q", r.Token, r.APIKey)
	}

	t.Setenv("AEGEAN_ENDPOINT", "local")
	t.Setenv("AEGEAN_TOKEN", "jwt-from-env")
	r, _ = Resolve(paths, "", "", "")
	if r.Endpoint != "http://localhost:3022" || r.Token != "jwt-from-env" {
		t.Errorf("env override failed: endpoint=%q token=%q", r.Endpoint, r.Token)
	}

	r, _ = Resolve(paths, "prod", "", "json")
	if r.Endpoint != "https://api.aegeanengine.com" || r.Output != "json" {
		t.Errorf("flag override failed: endpoint=%q output=%q", r.Endpoint, r.Output)
	}
}

func TestSetAndDeleteProfile(t *testing.T) {
	paths := tempPaths(t)
	if err := SetProfile(paths, "alice", Profile{Token: "t1", Email: "alice@x.io"}); err != nil {
		t.Fatalf("SetProfile alice: %v", err)
	}
	if err := SetProfile(paths, "bob", Profile{Token: "t2"}); err != nil {
		t.Fatalf("SetProfile bob: %v", err)
	}
	creds, _ := LoadCredentials(paths)
	if len(creds.Profiles) != 2 {
		t.Errorf("want 2 profiles, got %d", len(creds.Profiles))
	}
	if err := DeleteProfile(paths, "alice"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	creds, _ = LoadCredentials(paths)
	if _, exists := creds.Profiles["alice"]; exists {
		t.Errorf("alice still present after delete")
	}
	if _, exists := creds.Profiles["bob"]; !exists {
		t.Errorf("bob removed accidentally")
	}
}
