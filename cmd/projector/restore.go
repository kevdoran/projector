package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

func newRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore [project]",
		Short: "Restore an archived project (recreates worktrees)",
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

			if p.Project.Status == project.StatusActive {
				return fmt.Errorf("project %q is already active", p.Project.Name)
			}

			if len(p.ArchivedWorktrees) == 0 {
				fmt.Println("No archived worktrees to restore.")
			}

			now := time.Now().UTC()
			var restored, skipped []string

			for _, record := range p.ArchivedWorktrees {
				// Check if the repo still exists on disk
				if _, err := os.Stat(record.RepoPath); err != nil {
					fmt.Fprintf(os.Stderr, "warning: repo %s not found at %s, skipping\n", record.RepoName, record.RepoPath)
					skipped = append(skipped, record.RepoName)
					continue
				}

				if record.Branch == "" {
					// Detached worktree: restore in detached HEAD state.
					commitish := record.Commit
					if commitish == "" {
						commitish = "HEAD" // fallback for legacy records
					}
					if err := git.WorktreeAddDetached(record.RepoPath, record.WorktreePath, commitish); err != nil {
						fmt.Fprintf(os.Stderr, "warning: restore detached %s: %v, skipping\n", record.RepoName, err)
						skipped = append(skipped, record.RepoName)
						continue
					}
					fmt.Printf("  restored: %s → %s (detached)\n", record.RepoName, record.WorktreePath)
					restored = append(restored, record.RepoName)
					continue
				}

				// Check if the branch still exists
				branchExists, err := git.BranchExists(record.RepoPath, record.Branch)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: check branch for %s: %v, skipping\n", record.RepoName, err)
					skipped = append(skipped, record.RepoName)
					continue
				}

				if branchExists {
					// Re-add worktree at the existing branch (no -b)
					if err := git.WorktreeAdd(record.RepoPath, record.WorktreePath, "", record.Branch, false); err != nil {
						fmt.Fprintf(os.Stderr, "warning: restore %s: %v, skipping\n", record.RepoName, err)
						skipped = append(skipped, record.RepoName)
						continue
					}
				} else {
					// Branch gone: find new available name and create fresh branch
					branchName, err := git.AvailableBranchName(record.RepoPath, p.Project.Name, now)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: branch name for %s: %v, skipping\n", record.RepoName, err)
						skipped = append(skipped, record.RepoName)
						continue
					}
					if err := git.WorktreeAdd(record.RepoPath, record.WorktreePath, "HEAD", branchName, true); err != nil {
						fmt.Fprintf(os.Stderr, "warning: restore %s with new branch %s: %v, skipping\n", record.RepoName, branchName, err)
						skipped = append(skipped, record.RepoName)
						continue
					}
					fmt.Printf("  note: branch %q no longer exists for %s; created %q\n", record.Branch, record.RepoName, branchName)
				}

				fmt.Printf("  restored: %s → %s\n", record.RepoName, record.WorktreePath)
				restored = append(restored, record.RepoName)
			}

			// Update project status to active
			p.Project.Status = project.StatusActive
			p.Project.ArchivedAt = nil
			p.ArchivedWorktrees = nil

			if err := project.Save(p, projectDir); err != nil {
				return fmt.Errorf("save project config: %w", err)
			}

			fmt.Printf("\nProject %q restored.\n", p.Project.Name)
			if len(restored) > 0 {
				fmt.Printf("  restored repos (%d): ", len(restored))
				for i, r := range restored {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(r)
				}
				fmt.Println()
			}
			if len(skipped) > 0 {
				fmt.Printf("  skipped repos (%d): ", len(skipped))
				for i, r := range skipped {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(r)
				}
				fmt.Println()
			}

			return nil
		},
	}

	return cmd
}
