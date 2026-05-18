package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDomainsCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage custom sender domains",
	}
	cmd.AddCommand(
		newDomainsAddCmd(flags),
		newDomainsListCmd(flags),
		newDomainsVerifyCmd(flags),
		newDomainsChecksCmd(flags),
	)
	return cmd
}

func newDomainsAddCmd(flags *GlobalFlags) *cobra.Command {
	var (
		domainType string
		intent     string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new sender domain (returns DNS records to set)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			req := client.AddDomainRequest{
				DomainName: args[0],
				Type:       strings.ToUpper(domainType),
				Intent:     strings.ToUpper(intent),
			}
			info, err := sess.client.AddDomain(context.Background(), req)
			if err != nil {
				return err
			}
			return renderDomain(cmd, sess.cfg.Output, info)
		},
	}
	cmd.Flags().StringVar(&domainType, "type", "CUSTOM", "Domain type: CUSTOM | SUBDOMAIN")
	cmd.Flags().StringVar(&intent, "intent", "OUTBOUND", "Domain intent: OUTBOUND | INBOUND | BOTH")
	return cmd
}

func newDomainsListCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered sender domains",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			domains, err := sess.client.ListDomains(context.Background())
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), domains)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), domains)
			}
			if len(domains) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No domains on this account. Register one with `aegean domains add <name>`.")
				return nil
			}
			rows := make([][]string, len(domains))
			for i, d := range domains {
				verified := "no"
				if d.Verified {
					verified = "yes"
				}
				rows[i] = []string{d.ID, d.DomainName, d.Type, verified, d.CreatedAt.Format("2006-01-02")}
			}
			return output.Table(cmd.OutOrStdout(),
				[]string{"ID", "DOMAIN", "TYPE", "VERIFIED", "CREATED"},
				rows,
			)
		},
	}
}

func newDomainsVerifyCmd(flags *GlobalFlags) *cobra.Command {
	var (
		timeout time.Duration
		every   time.Duration
	)
	cmd := &cobra.Command{
		Use:   "verify <name|id>",
		Short: "Trigger and poll DKIM/SPF/DMARC verification until it passes or times out",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}

			id, err := resolveDomainID(sess.client, args[0])
			if err != nil {
				return err
			}

			deadline := time.Now().Add(timeout)
			var info *client.DomainInfo
			for attempt := 1; ; attempt++ {
				info, err = sess.client.VerifyDomain(context.Background(), id)
				if err == nil && info.Verified {
					break
				}
				if err != nil {
					var apiErr *client.ErrAPI
					if errors.As(err, &apiErr) && apiErr.Status == 404 {
						return fmt.Errorf("domain %s not found", args[0])
					}
				}
				if time.Now().After(deadline) {
					if err == nil {
						err = fmt.Errorf("verification did not complete within %s (DNS may still be propagating)", timeout)
					}
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  attempt %d: not verified yet — retrying in %s\n", attempt, every)
				time.Sleep(every)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Verified %s ✓\n", info.DomainName)
			return renderDomain(cmd, sess.cfg.Output, info)
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Give up after this much time")
	cmd.Flags().DurationVar(&every, "every", 15*time.Second, "Poll interval between verify attempts")
	return cmd
}

func newDomainsChecksCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "checks <name|id>",
		Short: "Show current DKIM/SPF/DMARC/MX status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			id, err := resolveDomainID(sess.client, args[0])
			if err != nil {
				return err
			}
			records, err := sess.client.DomainChecks(context.Background(), id)
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), records)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), records)
			}
			if len(records) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No DNS checks yet.")
				return nil
			}
			rows := make([][]string, len(records))
			for i, r := range records {
				rows[i] = []string{r.Type, r.Host, r.Record, verifiedLabel(r.Verified, r.Required), truncateCell(r.Value, 60)}
			}
			return output.Table(cmd.OutOrStdout(),
				[]string{"TYPE", "HOST", "DNS", "STATUS", "VALUE"},
				rows,
			)
		},
	}
}

func resolveDomainID(c *client.Client, nameOrID string) (string, error) {
	if looksLikeUUID(nameOrID) {
		return nameOrID, nil
	}
	d, err := c.FindDomainByName(context.Background(), nameOrID)
	if err != nil {
		return "", err
	}
	return d.ID, nil
}

func looksLikeUUID(s string) bool {
	// Cheap heuristic; the backend does the authoritative parse.
	return len(s) == 36 && strings.Count(s, "-") == 4
}

func renderDomain(cmd *cobra.Command, format string, info *client.DomainInfo) error {
	switch format {
	case output.FormatJSON:
		return output.JSON(cmd.OutOrStdout(), info)
	case output.FormatYAML:
		return output.YAML(cmd.OutOrStdout(), info)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Domain: %s\n  id:       %s\n  type:     %s\n  verified: %t\n",
		info.DomainName, info.ID, info.Type, info.Verified)
	if len(info.DNSRecords) == 0 {
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\n  Required DNS records:")
	rows := make([][]string, len(info.DNSRecords))
	for i, r := range info.DNSRecords {
		rows[i] = []string{r.Type, r.Host, r.Record, verifiedLabel(r.Verified, r.Required), truncateCell(r.Value, 60)}
	}
	return output.Table(cmd.OutOrStdout(),
		[]string{"TYPE", "HOST", "DNS", "STATUS", "VALUE"},
		rows,
	)
}

func verifiedLabel(verified *bool, required bool) string {
	switch {
	case verified == nil:
		return "?"
	case *verified:
		return "ok"
	case required:
		return "MISSING"
	default:
		return "missing"
	}
}

func truncateCell(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
