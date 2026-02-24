package main

import "github.com/spf13/cobra"

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(
		newListCmd(),
		newCreateCmd(),
		newAddRepoCmd(),
		newArchiveCmd(),
		newRestoreCmd(),
	)
	return cmd
}
