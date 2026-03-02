package main

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage global configuration (~/.projector/projector-config.toml)",
	}
	cmd.AddCommand(
		newConfigSetupCmd(),
		newConfigListCmd(),
		newConfigGetCmd(),
		newConfigSetCmd(),
	)
	return cmd
}
