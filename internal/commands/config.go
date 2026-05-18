package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Aegean-Robotics/aegean-cli/internal/config"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

// Allowed config keys. Anything else is rejected by `config set` to keep the
// file shape predictable.
var configKeys = map[string]struct{}{
	"endpoint":        {},
	"default_profile": {},
	"output":          {},
	"telemetry":       {},
}

func newConfigCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and modify ~/.aegean/config",
	}
	cmd.AddCommand(
		newConfigGetCmd(flags),
		newConfigSetCmd(flags),
		newConfigListCmd(flags),
		newConfigUnsetCmd(flags),
	)
	return cmd
}

func newConfigGetCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return err
			}
			val, err := readConfigKey(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			_ = flags // kept for symmetry with other commands
			return nil
		},
	}
}

func newConfigSetCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update a config key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return err
			}
			if err := writeConfigKey(&cfg, args[0], args[1]); err != nil {
				return err
			}
			if err := config.SaveConfig(paths, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", args[0], args[1])
			_ = flags
			return nil
		},
	}
}

func newConfigUnsetCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key (revert to default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return err
			}
			if err := writeConfigKey(&cfg, args[0], ""); err != nil {
				return err
			}
			if err := config.SaveConfig(paths, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unset %s\n", args[0])
			_ = flags
			return nil
		},
	}
}

func newConfigListCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return err
			}
			r, err := config.Resolve(paths, flags.Endpoint, flags.Profile, flags.Output)
			if err != nil {
				return err
			}
			values := map[string]string{
				"endpoint":        firstNonEmptyStr(cfg.Endpoint, "(default: prod)"),
				"default_profile": firstNonEmptyStr(cfg.DefaultProfile, "(default: "+config.DefaultProfile+")"),
				"output":          firstNonEmptyStr(cfg.Output, "(default: "+config.DefaultOutput+")"),
				"telemetry":       strconv.FormatBool(cfg.Telemetry),
			}
			switch r.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), values)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), values)
			}
			keys := make([]string, 0, len(values))
			for k := range values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			rows := make([][]string, len(keys))
			for i, k := range keys {
				rows[i] = []string{k, values[k]}
			}
			return output.Table(cmd.OutOrStdout(), []string{"KEY", "VALUE"}, rows)
		},
	}
}

func readConfigKey(cfg config.Config, key string) (string, error) {
	switch key {
	case "endpoint":
		return cfg.Endpoint, nil
	case "default_profile":
		return cfg.DefaultProfile, nil
	case "output":
		return cfg.Output, nil
	case "telemetry":
		return strconv.FormatBool(cfg.Telemetry), nil
	}
	return "", fmt.Errorf("unknown config key %q (allowed: %s)", key, allowedKeysList())
}

func writeConfigKey(cfg *config.Config, key, value string) error {
	if _, ok := configKeys[key]; !ok {
		return fmt.Errorf("unknown config key %q (allowed: %s)", key, allowedKeysList())
	}
	switch key {
	case "endpoint":
		if value == "" {
			cfg.Endpoint = ""
			return nil
		}
		if _, err := config.ResolveEndpoint(value); err != nil {
			return err
		}
		cfg.Endpoint = value
	case "default_profile":
		cfg.DefaultProfile = value
	case "output":
		if value == "" {
			cfg.Output = ""
			return nil
		}
		if err := output.Validate(value); err != nil {
			return err
		}
		cfg.Output = value
	case "telemetry":
		if value == "" {
			cfg.Telemetry = false
			return nil
		}
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("telemetry: %w (use true|false)", err)
		}
		cfg.Telemetry = b
	}
	return nil
}

func allowedKeysList() string {
	keys := make([]string, 0, len(configKeys))
	for k := range configKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
