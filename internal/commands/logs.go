package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLogsCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read transactional email logs",
	}
	cmd.AddCommand(newLogsListCmd(flags), newLogsTailCmd(flags))
	return cmd
}

func newLogsListCmd(flags *GlobalFlags) *cobra.Command {
	var (
		page int
		size int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent log entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireAPIKey(); err != nil {
				return err
			}
			pageData, err := sess.withAPIKey().ListLogs(context.Background(), page, size)
			if err != nil {
				return err
			}
			return renderLogs(cmd, sess.cfg.Output, pageData)
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "Page index (0-based)")
	cmd.Flags().IntVar(&size, "size", 20, "Page size (max 100)")
	return cmd
}

func newLogsTailCmd(flags *GlobalFlags) *cobra.Command {
	var (
		interval time.Duration
		size     int
	)
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream new log entries until ctrl-C",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireAPIKey(); err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()
			c := sess.withAPIKey()

			seen := map[string]struct{}{}
			first := true
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				pageData, err := c.ListLogs(ctx, 0, size)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "tail: %v\n", err)
				} else {
					var fresh []client.EmailLog
					for _, l := range pageData.Content {
						if _, ok := seen[l.ID]; ok {
							continue
						}
						seen[l.ID] = struct{}{}
						if first {
							continue // skip the existing tail on first poll
						}
						fresh = append(fresh, l)
					}
					first = false
					for _, l := range fresh {
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s → %s — %s\n",
							safeTime(l.SentAt), l.Status, l.Recipient, l.Subject)
					}
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")
	cmd.Flags().IntVar(&size, "size", 20, "Page size per poll")
	return cmd
}

func renderLogs(cmd *cobra.Command, format string, page *client.LogsPage) error {
	switch format {
	case output.FormatJSON:
		return output.JSON(cmd.OutOrStdout(), page)
	case output.FormatYAML:
		return output.YAML(cmd.OutOrStdout(), page)
	}
	if len(page.Content) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No log entries yet.")
		return nil
	}
	rows := make([][]string, len(page.Content))
	for i, l := range page.Content {
		rows[i] = []string{safeTime(l.SentAt), l.Status, l.Recipient, truncateCell(l.Subject, 50)}
	}
	if err := output.Table(cmd.OutOrStdout(),
		[]string{"SENT", "STATUS", "RECIPIENT", "SUBJECT"},
		rows,
	); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nshowing page %d of %d (%d total)\n",
		page.Number+1, page.TotalPages, page.TotalElements)
	return nil
}

func safeTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
