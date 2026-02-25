package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

func newDescCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "desc [project]",
		Short: "Show details for a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			projectDir, p, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			if verbose {
				return descVerbose(projectDir, p)
			}
			return descSummary(projectDir, p)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show git status details for each worktree")
	return cmd
}

// worktreeStatus pairs a live WorktreeInfo with its git status.
type worktreeStatus struct {
	info  project.WorktreeInfo
	clean bool
	lines []string // git status --short lines, populated when dirty
}

// collectWorktreeStatuses gathers live worktree info and git status for active projects,
// or returns the archived worktree records (without git status) for archived ones.
func collectWorktreeStatuses(projectDir string, p *project.ProjectConfig) ([]worktreeStatus, error) {
	if p.Project.Status != project.StatusActive {
		var statuses []worktreeStatus
		for _, wt := range p.ArchivedWorktrees {
			statuses = append(statuses, worktreeStatus{
				info: project.WorktreeInfo{
					RepoName:     wt.RepoName,
					RepoPath:     wt.RepoPath,
					WorktreePath: wt.WorktreePath,
					Branch:       wt.Branch,
				},
			})
		}
		return statuses, nil
	}

	worktrees, err := project.DiscoverWorktrees(projectDir)
	if err != nil {
		return nil, fmt.Errorf("discover worktrees: %w", err)
	}

	var statuses []worktreeStatus
	for _, wt := range worktrees {
		clean, lines, err := git.StatusPorcelain(wt.WorktreePath)
		if err != nil {
			statuses = append(statuses, worktreeStatus{info: wt, lines: []string{"(status unavailable)"}})
			continue
		}
		statuses = append(statuses, worktreeStatus{info: wt, clean: clean, lines: lines})
	}
	return statuses, nil
}

func descSummary(projectDir string, p *project.ProjectConfig) error {
	statuses, err := collectWorktreeStatuses(projectDir, p)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "REPO\tBRANCH\tSTATUS")
	for _, s := range statuses {
		var statusStr string
		switch {
		case p.Project.Status != project.StatusActive:
			statusStr = "archived"
		case s.clean:
			statusStr = "clean"
		default:
			statusStr = fmt.Sprintf("dirty (%d)", len(s.lines))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.info.RepoName, s.info.Branch, statusStr)
	}
	w.Flush()
	return nil
}

func descVerbose(projectDir string, p *project.ProjectConfig) error {
	statuses, err := collectWorktreeStatuses(projectDir, p)
	if err != nil {
		return err
	}

	// Project header — use a tabwriter so values align.
	hw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(hw, "Project:\t%s\n", p.Project.Name)
	fmt.Fprintf(hw, "Path:\t%s\n", projectDir)
	fmt.Fprintf(hw, "Status:\t%s\n", p.Project.Status)
	fmt.Fprintf(hw, "Created:\t%s\n", humanizeTime(p.Project.CreatedAt))
	hw.Flush()

	// Per-repo blocks.
	for _, s := range statuses {
		fmt.Println()

		dirty := p.Project.Status == project.StatusActive && !s.clean
		if dirty {
			fmt.Fprintf(os.Stdout, "%s  [%s]  dirty\n", s.info.RepoName, s.info.Branch)
		} else {
			fmt.Fprintf(os.Stdout, "%s  [%s]\n", s.info.RepoName, s.info.Branch)
		}

		// %-6s aligns "path" (4) and "status" (6) to the same value column.
		fmt.Fprintf(os.Stdout, "  %-6s  %s\n", "path", s.info.WorktreePath)

		switch {
		case p.Project.Status != project.StatusActive:
			fmt.Fprintf(os.Stdout, "  %-6s  archived\n", "status")
		case s.clean:
			fmt.Fprintf(os.Stdout, "  %-6s  clean\n", "status")
		default:
			fmt.Fprintf(os.Stdout, "  %-6s  dirty\n", "status")
			for _, line := range s.lines {
				fmt.Fprintf(os.Stdout, "    %s\n", line)
			}
		}
	}
	return nil
}
