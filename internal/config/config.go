// Package config loads and persists CLI configuration and credentials.
//
// Two files under ~/.aegean/:
//
//	config       0644 — endpoint, default profile, output format, telemetry
//	credentials  0600 — JWT, account alias, optional API key
//
// Precedence for any resolved value (highest wins):
//
//	1. explicit CLI flag
//	2. AEGEAN_* environment variable
//	3. value in the profile file
//	4. built-in default
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	dirName            = ".aegean"
	configFileName     = "config"
	credentialsName    = "credentials"
	configFileMode     = 0o644
	credentialsMode    = 0o600
	DefaultProfile     = "default"
	DefaultOutput      = "text"
	defaultEndpointKey = "prod"
)

// Endpoints baked in. Override with --endpoint <prod|dev|local|https://...>.
var Endpoints = map[string]string{
	"prod":  "https://api.aegeanengine.com",
	"dev":   "https://api.dev.aegeanengine.com",
	"local": "http://localhost:3022",
}

// Config maps ~/.aegean/config (TOML).
type Config struct {
	Endpoint       string `toml:"endpoint"`
	DefaultProfile string `toml:"default_profile"`
	Output         string `toml:"output"`
	Telemetry      bool   `toml:"telemetry"`
}

// Credentials maps ~/.aegean/credentials (TOML). Multi-profile.
type Credentials struct {
	Profiles map[string]Profile `toml:"profiles"`
}

// Profile is one named identity in credentials.
type Profile struct {
	Token        string `toml:"token"`
	AccountAlias string `toml:"account_alias,omitempty"`
	Email        string `toml:"email,omitempty"`
	APIKey       string `toml:"api_key,omitempty"`
}

// Resolved is what commands actually read after layering flag/env/file.
type Resolved struct {
	Endpoint  string
	Profile   string
	Output    string
	Telemetry bool
	Token     string
	APIKey    string
	Email     string
	Alias     string
}

// Paths exposes the on-disk locations. Used by commands and tests.
type Paths struct {
	Dir         string
	Config      string
	Credentials string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("locate home directory: %w", err)
	}
	dir := filepath.Join(home, dirName)
	return Paths{
		Dir:         dir,
		Config:      filepath.Join(dir, configFileName),
		Credentials: filepath.Join(dir, credentialsName),
	}, nil
}

// LoadConfig reads ~/.aegean/config. Missing file returns zero-value (not an error).
func LoadConfig(paths Paths) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(paths.Config)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", paths.Config, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", paths.Config, err)
	}
	return cfg, nil
}

// SaveConfig writes ~/.aegean/config atomically.
func SaveConfig(paths Paths, cfg Config) error {
	if err := ensureDir(paths.Dir); err != nil {
		return err
	}
	return writeAtomic(paths.Config, marshalTOML(cfg), configFileMode)
}

// LoadCredentials reads ~/.aegean/credentials. Refuses to read if the file is
// world- or group-readable on POSIX — credentials must be 0600.
func LoadCredentials(paths Paths) (Credentials, error) {
	creds := Credentials{Profiles: map[string]Profile{}}
	info, err := os.Stat(paths.Credentials)
	if errors.Is(err, os.ErrNotExist) {
		return creds, nil
	}
	if err != nil {
		return creds, fmt.Errorf("stat %s: %w", paths.Credentials, err)
	}
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			return creds, fmt.Errorf(
				"credentials file %s has insecure permissions %o; expected 0600. "+
					"run: chmod 600 %s",
				paths.Credentials, mode, paths.Credentials,
			)
		}
	}
	data, err := os.ReadFile(paths.Credentials)
	if err != nil {
		return creds, fmt.Errorf("read %s: %w", paths.Credentials, err)
	}
	if err := toml.Unmarshal(data, &creds); err != nil {
		return creds, fmt.Errorf("parse %s: %w", paths.Credentials, err)
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]Profile{}
	}
	return creds, nil
}

// SaveCredentials writes ~/.aegean/credentials atomically with 0600 perms.
func SaveCredentials(paths Paths, creds Credentials) error {
	if err := ensureDir(paths.Dir); err != nil {
		return err
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]Profile{}
	}
	return writeAtomic(paths.Credentials, marshalTOML(creds), credentialsMode)
}

// Resolve folds all sources together.
//
// flagEndpoint, flagProfile, flagOutput are the values passed in by cobra
// (empty string means "not set").
func Resolve(paths Paths, flagEndpoint, flagProfile, flagOutput string) (Resolved, error) {
	cfg, err := LoadConfig(paths)
	if err != nil {
		return Resolved{}, err
	}
	creds, err := LoadCredentials(paths)
	if err != nil {
		return Resolved{}, err
	}

	profile := firstNonEmpty(flagProfile, os.Getenv("AEGEAN_PROFILE"), cfg.DefaultProfile, DefaultProfile)
	endpoint := firstNonEmpty(flagEndpoint, os.Getenv("AEGEAN_ENDPOINT"), cfg.Endpoint, defaultEndpointKey)
	output := firstNonEmpty(flagOutput, os.Getenv("AEGEAN_OUTPUT"), cfg.Output, DefaultOutput)

	endpointURL, err := ResolveEndpoint(endpoint)
	if err != nil {
		return Resolved{}, err
	}

	p := creds.Profiles[profile]
	if envToken := os.Getenv("AEGEAN_TOKEN"); envToken != "" {
		p.Token = envToken
	}
	if envKey := os.Getenv("AEGEAN_API_KEY"); envKey != "" {
		p.APIKey = envKey
	}

	return Resolved{
		Endpoint:  endpointURL,
		Profile:   profile,
		Output:    output,
		Telemetry: cfg.Telemetry,
		Token:     p.Token,
		APIKey:    p.APIKey,
		Email:     p.Email,
		Alias:     p.AccountAlias,
	}, nil
}

// ResolveEndpoint turns prod|dev|local|<url> into a fully-qualified URL.
func ResolveEndpoint(name string) (string, error) {
	if name == "" {
		return Endpoints[defaultEndpointKey], nil
	}
	if url, ok := Endpoints[name]; ok {
		return url, nil
	}
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return strings.TrimRight(name, "/"), nil
	}
	return "", fmt.Errorf("unknown endpoint %q (use prod|dev|local or an http(s):// URL)", name)
}

// SetProfile inserts or updates a single profile and persists.
func SetProfile(paths Paths, profile string, p Profile) error {
	creds, err := LoadCredentials(paths)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Permissions issue or parse error — surface it.
		return err
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]Profile{}
	}
	creds.Profiles[profile] = p
	return SaveCredentials(paths, creds)
}

// DeleteProfile removes a single profile and persists.
func DeleteProfile(paths Paths, profile string) error {
	creds, err := LoadCredentials(paths)
	if err != nil {
		return err
	}
	delete(creds.Profiles, profile)
	return SaveCredentials(paths, creds)
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	return nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".aegean-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}

func marshalTOML(v any) []byte {
	var sb strings.Builder
	enc := toml.NewEncoder(&sb)
	enc.Indent = ""
	if err := enc.Encode(v); err != nil {
		// Encoding a small typed struct cannot fail in practice.
		panic(fmt.Sprintf("toml encode: %v", err))
	}
	return []byte(sb.String())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
