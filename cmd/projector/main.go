package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/git"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pj",
		Short: "📽️  pj — manage parallel projects backed by git worktrees",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip version check for help flags
			if cmd.Name() == "help" {
				return nil
			}
			return git.MinVersionCheck()
		},
	}

	root.Version = version
	root.SetVersionTemplate(versionString())

	root.AddCommand(newProjectsCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd())

	// Rename Cobra's default "completion" command to "autocomp".
	// InitDefaultCompletionCmd adds it now so we can rename it before Execute()
	// re-runs the same logic. DisableDefaultCmd=true prevents Execute() from
	// adding a second "completion" command on top of the renamed one.
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "completion" {
			cmd.Use = "autocomp"
			break
		}
	}
	root.CompletionOptions.DisableDefaultCmd = true

	return root
}

// loadConfig loads the global config, returning an error if not found.
func loadConfig() (*config.GlobalConfig, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	if err == config.ErrNotFound {
		return nil, fmt.Errorf("no configuration found; run 'pj config setup' to set up pj")
	}
	return nil, fmt.Errorf("load config: %w", err)
}
