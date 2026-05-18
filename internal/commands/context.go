package commands

import (
	"fmt"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/config"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
)

// session is the resolved state every command operates against.
type session struct {
	cfg    config.Resolved
	paths  config.Paths
	client *client.Client
}

// loadSession resolves config + credentials and returns a client primed with
// the JWT (if present). API key is left unset — commands that need it call
// requireAPIKey.
func loadSession(flags *GlobalFlags) (*session, error) {
	if err := output.Validate(flags.Output); err != nil {
		return nil, err
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	r, err := config.Resolve(paths, flags.Endpoint, flags.Profile, flags.Output)
	if err != nil {
		return nil, err
	}
	c := client.New(r.Endpoint)
	if r.Token != "" {
		c = c.WithToken(r.Token)
	}
	return &session{cfg: r, paths: paths, client: c}, nil
}

func (s *session) requireToken() error {
	if s.cfg.Token == "" {
		return fmt.Errorf("not logged in — run `aegean login` first (or set AEGEAN_TOKEN)")
	}
	return nil
}

func (s *session) requireAPIKey() error {
	if s.cfg.APIKey == "" {
		return fmt.Errorf(
			"no API key configured — create one with `aegean keys create <label>` " +
				"(or set AEGEAN_API_KEY)",
		)
	}
	return nil
}

// withAPIKey returns a client primed with both the JWT and the API key.
// Email send + log read use this.
func (s *session) withAPIKey() *client.Client {
	return s.client.WithAPIKey(s.cfg.APIKey)
}
