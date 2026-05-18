package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLoginCmd(flags *GlobalFlags) *cobra.Command {
	var (
		emailFlag    string
		passwordFlag string
		aliasFlag    string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Aegean API and store credentials",
		Long: "Interactive login: prompts for email + password (and optional account alias).\n" +
			"On success, writes ~/.aegean/credentials (mode 0600) with the resulting JWT.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}

			email := emailFlag
			alias := aliasFlag
			password := passwordFlag

			if email == "" {
				email, err = prompt(cmd.OutOrStdout(), cmd.InOrStdin(), "Email: ")
				if err != nil {
					return err
				}
			}
			if alias == "" {
				alias, err = prompt(cmd.OutOrStdout(), cmd.InOrStdin(), "Account alias (optional, press enter to auto-resolve): ")
				if err != nil {
					return err
				}
			}
			if password == "" {
				password, err = promptPassword(cmd.OutOrStdout(), "Password: ")
				if err != nil {
					return err
				}
			}

			resp, err := sess.client.Login(context.Background(), client.LoginRequest{
				AccountAlias: alias,
				Email:        email,
				Password:     password,
			})
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			profile := sess.cfg.Profile
			existing := config.Profile{}
			if creds, err := config.LoadCredentials(sess.paths); err == nil {
				if p, ok := creds.Profiles[profile]; ok {
					existing = p
				}
			}
			existing.Token = resp.Token
			existing.Email = resp.Email
			if alias != "" {
				existing.AccountAlias = alias
			}
			if err := config.SetProfile(sess.paths, profile, existing); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Logged in as %s (account %s, role %s) — credentials written to %s\n",
				resp.Email, resp.AccountID, resp.Role, sess.paths.Credentials,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&emailFlag, "email", "", "Email (non-interactive)")
	cmd.Flags().StringVar(&passwordFlag, "password", "", "Password (non-interactive). Prefer AEGEAN_PASSWORD env to avoid shell history.")
	cmd.Flags().StringVar(&aliasFlag, "account", "", "Account alias (optional)")
	return cmd
}

func newLogoutCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Discard stored credentials for the current profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := config.DeleteProfile(sess.paths, sess.cfg.Profile); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged out (profile %q removed from %s).\n", sess.cfg.Profile, sess.paths.Credentials)
			return nil
		},
	}
}

func prompt(out io.Writer, in io.Reader, label string) (string, error) {
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func promptPassword(out io.Writer, label string) (string, error) {
	fd := int(syscall.Stdin)
	if !term.IsTerminal(fd) {
		// Non-interactive: read one line from stdin.
		return prompt(out, os.Stdin, label)
	}
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", err
	}
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}
