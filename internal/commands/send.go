package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSendCmd(flags *GlobalFlags) *cobra.Command {
	var (
		to       string
		from     string
		subject  string
		text     string
		html     string
		htmlFile string
		replyTo  string
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a transactional email",
		Long: "Send a transactional email through /v1/email/send.\n\n" +
			"Body precedence: --html-file > --html > --text. At least one of those\n" +
			"three flags is required.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireAPIKey(); err != nil {
				return err
			}
			if to == "" {
				return fmt.Errorf("--to is required")
			}
			if subject == "" {
				return fmt.Errorf("--subject is required")
			}

			body, err := resolveBody(text, html, htmlFile)
			if err != nil {
				return err
			}

			resp, err := sess.withAPIKey().SendEmail(context.Background(), client.SendEmailRequest{
				To:      to,
				Subject: subject,
				HTML:    body,
				From:    from,
				ReplyTo: replyTo,
			})
			if err != nil {
				return err
			}

			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), resp)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "→ message id: %s\n→ status:     %s\n", resp.ID, resp.Status)
			if resp.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "→ error:      %s\n", resp.Error)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Recipient address (required)")
	cmd.Flags().StringVar(&from, "from", "", "Sender (must belong to a verified domain)")
	cmd.Flags().StringVar(&subject, "subject", "", "Subject line (required)")
	cmd.Flags().StringVar(&text, "text", "", "Plain-text body (wrapped in <pre> for HTML transport)")
	cmd.Flags().StringVar(&html, "html", "", "HTML body literal")
	cmd.Flags().StringVar(&htmlFile, "html-file", "", "Read HTML body from this file (use - for stdin)")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply-To address")
	return cmd
}

func resolveBody(text, html, htmlFile string) (string, error) {
	if htmlFile != "" {
		var r io.Reader
		if htmlFile == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(htmlFile)
			if err != nil {
				return "", fmt.Errorf("open --html-file: %w", err)
			}
			defer f.Close()
			r = f
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return "", fmt.Errorf("read --html-file: %w", err)
		}
		return string(data), nil
	}
	if html != "" {
		return html, nil
	}
	if text != "" {
		return "<pre style=\"font-family:inherit;margin:0;white-space:pre-wrap\">" + escapeHTML(text) + "</pre>", nil
	}
	return "", fmt.Errorf("one of --text, --html, or --html-file is required")
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
