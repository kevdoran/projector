package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

func newRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair [project]",
		Short: "Repair git worktrees after a project was moved or renamed",
		Long: "Repair the git worktrees in a project. Moving or renaming a project " +
			"directory leaves each worktree's gitdir/commondir pointers stale; this " +
			"runs 'git worktree repair' for every worktree so git can find them again.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			projectDir, p, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			if p.Project.Status != project.StatusActive {
				return fmt.Errorf("project %q is not active (status: %s); only active projects have live worktrees to repair", p.Project.Name, p.Project.Status)
			}

			worktrees, err := project.DiscoverWorktrees(projectDir)
			if err != nil {
				return fmt.Errorf("discover worktrees: %w", err)
			}

			if len(worktrees) == 0 {
				fmt.Printf("Project %q has no worktrees to repair.\n", p.Project.Name)
				return nil
			}

			var failed []string
			for _, wt := range worktrees {
				fmt.Printf("  Repairing worktree for %s...\n", wt.RepoName)
				// Run repair from the underlying repo, passing the worktree path so
				// git can locate it even when its directory was moved.
				if err := git.WorktreeRepair(wt.RepoPath, wt.WorktreePath); err != nil {
					failed = append(failed, wt.RepoName)
					fmt.Printf("    failed: %v\n", err)
					continue
				}
				fmt.Printf("    repaired: %s\n", wt.WorktreePath)
			}

			if len(failed) > 0 {
				return fmt.Errorf("failed to repair %d worktree(s): %v", len(failed), failed)
			}

			fmt.Printf("Repaired %d worktree(s) in project %q.\n", len(worktrees), p.Project.Name)
			return nil
		},
	}

	return cmd
}
