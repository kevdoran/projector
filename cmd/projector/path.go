package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path [project]",
		Short: "Print the absolute path to a project directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			projectDir, _, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			fmt.Println(projectDir)
			return nil
		},
	}
}
