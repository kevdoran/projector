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
		newDescCmd(),
		newOpenCmd(),
		newPathCmd(),
		newAddRepoCmd(),
		newArchiveCmd(),
		newRestoreCmd(),
		newDeleteCmd(),
	)
	return cmd
}
