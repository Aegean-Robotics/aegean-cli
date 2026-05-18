package commands

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/config"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

func newKeysCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
	}
	cmd.AddCommand(
		newKeysCreateCmd(flags),
		newKeysListCmd(flags),
		newKeysDeleteCmd(flags),
	)
	return cmd
}

func newKeysCreateCmd(flags *GlobalFlags) *cobra.Command {
	var (
		rateLimit int
		save      bool
	)
	cmd := &cobra.Command{
		Use:   "create <label>",
		Short: "Create a new API key (the secret is shown ONCE — copy it now)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			req := client.CreateKeyRequest{Name: args[0]}
			if rateLimit > 0 {
				req.RateLimit = &rateLimit
			}
			resp, err := sess.client.CreateAPIKey(context.Background(), req)
			if err != nil {
				return err
			}

			switch sess.cfg.Output {
			case output.FormatJSON:
				if err := output.JSON(cmd.OutOrStdout(), resp); err != nil {
					return err
				}
			case output.FormatYAML:
				if err := output.YAML(cmd.OutOrStdout(), resp); err != nil {
					return err
				}
			default:
				fmt.Fprintf(cmd.OutOrStdout(),
					"Created API key %q\n  id:     %s\n  secret: %s\n\n"+
						"This is the only time the full secret is shown — copy it now.\n",
					resp.Name, resp.ID, resp.Key,
				)
			}

			if save {
				profile := sess.cfg.Profile
				creds, _ := config.LoadCredentials(sess.paths)
				p := creds.Profiles[profile]
				p.APIKey = resp.Key
				if err := config.SetProfile(sess.paths, profile, p); err != nil {
					return fmt.Errorf("save key to profile: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Saved to profile %q in %s (used by `aegean send`).\n", profile, sess.paths.Credentials)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "Per-key rate limit (requests/min); 0 = server default")
	cmd.Flags().BoolVar(&save, "save", false, "Persist the new key to ~/.aegean/credentials so `aegean send` can use it")
	return cmd
}

func newKeysListCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List API keys for the current account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			keys, err := sess.client.ListAPIKeys(context.Background())
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), keys)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), keys)
			}
			if len(keys) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No API keys on this account. Create one with `aegean keys create <label>`.")
				return nil
			}
			rows := make([][]string, len(keys))
			for i, k := range keys {
				last := "never"
				if k.LastUsedAt != nil {
					last = k.LastUsedAt.Format("2006-01-02 15:04")
				}
				rows[i] = []string{k.ID, k.Name, k.KeyPrefix + "…", last, k.CreatedAt.Format("2006-01-02")}
			}
			return output.Table(cmd.OutOrStdout(),
				[]string{"ID", "NAME", "PREFIX", "LAST USED", "CREATED"},
				rows,
			)
		},
	}
}

func newKeysDeleteCmd(flags *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "Delete API key %s? Type 'yes' to confirm: ", args[0])
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := sess.client.DeleteAPIKey(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted API key %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}
