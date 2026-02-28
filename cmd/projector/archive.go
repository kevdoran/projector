package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

func newArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive [project]",
		Short: "Archive an active project (removes worktrees, keeps branch)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			projectDir, p, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			if p.Project.Status != project.StatusActive {
				return fmt.Errorf("project %q is not active (status: %s)", p.Project.Name, p.Project.Status)
			}

			// Discover live worktrees
			worktrees, err := project.DiscoverWorktrees(projectDir)
			if err != nil {
				return fmt.Errorf("discover worktrees: %w", err)
			}

			// Pre-validate: check all worktrees are clean and capture HEAD SHAs
			// for detached worktrees (must be done before removal).
			var dirtyRepos []string
			detachedSHAs := map[string]string{} // worktreePath → SHA
			for _, wt := range worktrees {
				clean, lines, err := git.StatusPorcelain(wt.WorktreePath)
				if err != nil {
					return fmt.Errorf("check status of %s: %w", wt.RepoName, err)
				}
				if !clean {
					dirtyRepos = append(dirtyRepos, fmt.Sprintf("  %s (%d modified files)", wt.RepoName, len(lines)))
				}
				if wt.Branch == "" {
					sha, err := git.HeadSHA(wt.WorktreePath)
					if err != nil {
						return fmt.Errorf("get HEAD SHA for detached worktree %s: %w", wt.RepoName, err)
					}
					detachedSHAs[wt.WorktreePath] = sha
				}
			}
			if len(dirtyRepos) > 0 {
				fmt.Fprintln(os.Stderr, "Cannot archive: the following repos have uncommitted changes:")
				for _, r := range dirtyRepos {
					fmt.Fprintln(os.Stderr, r)
				}
				return fmt.Errorf("commit or stash changes before archiving")
			}

			// Execute: remove worktrees, tracking success for rollback
			type removedWorktree struct {
				wt project.WorktreeInfo
			}
			var removed []removedWorktree
			var removeErr error

			for _, wt := range worktrees {
				if err := git.WorktreeRemove(wt.RepoPath, wt.WorktreePath); err != nil {
					removeErr = fmt.Errorf("remove worktree %s: %w", wt.RepoName, err)
					break
				}
				if err := os.RemoveAll(wt.WorktreePath); err != nil {
					removeErr = fmt.Errorf("remove worktree dir %s: %w", wt.WorktreePath, err)
					break
				}
				removed = append(removed, removedWorktree{wt})
				fmt.Printf("  removed worktree: %s\n", wt.WorktreePath)
			}

			if removeErr != nil {
				fmt.Fprintf(os.Stderr, "error during archive: %v\n", removeErr)
				fmt.Fprintln(os.Stderr, "Attempting rollback...")

				// Rollback: re-add removed worktrees (branch still exists)
				var rollbackFailed []string
				for _, r := range removed {
					wt := r.wt
					if err := git.WorktreeAdd(wt.RepoPath, wt.WorktreePath, "", wt.Branch, false); err != nil {
						rollbackFailed = append(rollbackFailed, wt.RepoName)
						fmt.Fprintf(os.Stderr, "  rollback failed for %s: %v\n", wt.RepoName, err)
					} else {
						fmt.Fprintf(os.Stderr, "  restored: %s\n", wt.RepoName)
					}
				}

				if len(rollbackFailed) > 0 {
					// Partial failure: mark as archive-failed
					now := time.Now().UTC()
					var records []project.WorktreeRecord
					for _, wt := range worktrees {
						records = append(records, project.WorktreeRecord{
							RepoName:     wt.RepoName,
							RepoPath:     wt.RepoPath,
							WorktreePath: wt.WorktreePath,
							Branch:       wt.Branch,
						})
					}
					p.Project.Status = project.StatusArchiveFailed
					p.Project.ArchivedAt = &now
					p.ArchivedWorktrees = records
					_ = project.Save(p, projectDir)

					fmt.Fprintln(os.Stderr, "\nProject marked as 'archive-failed'. Manual resolution required for:")
					for _, name := range rollbackFailed {
						fmt.Fprintf(os.Stderr, "  - %s\n", name)
					}
					fmt.Fprintln(os.Stderr, "Run 'pj project restore' to attempt recovery.")
				}

				return removeErr
			}

			// Success: write archived state
			now := time.Now().UTC()
			var records []project.WorktreeRecord
			for _, wt := range worktrees {
				record := project.WorktreeRecord{
					RepoName:     wt.RepoName,
					RepoPath:     wt.RepoPath,
					WorktreePath: wt.WorktreePath,
					Branch:       wt.Branch,
				}
				if sha, ok := detachedSHAs[wt.WorktreePath]; ok {
					record.Commit = sha
				}
				records = append(records, record)
			}
			p.Project.Status = project.StatusArchived
			p.Project.ArchivedAt = &now
			p.ArchivedWorktrees = records

			if err := project.Save(p, projectDir); err != nil {
				return fmt.Errorf("save project config: %w", err)
			}

			fmt.Printf("Project %q archived.\n", p.Project.Name)
			return nil
		},
	}

	return cmd
}
