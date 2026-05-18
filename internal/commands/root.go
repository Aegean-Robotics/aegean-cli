package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type GlobalFlags struct {
	Endpoint string
	Profile  string
	Output   string
	Verbose  bool
}

func NewRoot(info BuildInfo) *cobra.Command {
	var flags GlobalFlags

	root := &cobra.Command{
		Use:           "aegean",
		Short:         "Aegean Cloud Engine command-line interface",
		Long:          "aegean — manage accounts, API keys, custom domains, and send transactional email/SMS/voice from the command line.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.Endpoint, "endpoint", "", "API endpoint (prod|dev|local|<url>) — overrides AEGEAN_ENDPOINT and config file")
	root.PersistentFlags().StringVar(&flags.Profile, "profile", "", "Named profile in ~/.aegean/credentials (default: \"default\")")
	root.PersistentFlags().StringVarP(&flags.Output, "output", "o", "text", "Output format: text|json|yaml")
	root.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Verbose output (HTTP requests, debug logs)")

	root.AddCommand(newVersionCmd(info))
	root.AddCommand(newLoginCmd(&flags))
	root.AddCommand(newLogoutCmd(&flags))
	root.AddCommand(newKeysCmd(&flags))
	root.AddCommand(newDomainsCmd(&flags))
	root.AddCommand(newSendCmd(&flags))
	root.AddCommand(newLogsCmd(&flags))
	root.AddCommand(newConfigCmd(&flags))

	return root
}

func newVersionCmd(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version + build info",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "aegean %s (commit %s, built %s)\n", info.Version, info.Commit, info.Date)
		},
	}
}
