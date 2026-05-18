package main

import (
	"fmt"
	"os"

	"github.com/Aegean-Robotics/aegean-cli/internal/commands"
)

// Injected at link time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := commands.NewRoot(commands.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
