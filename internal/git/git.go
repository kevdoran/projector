// Package git provides wrappers around the git executable for worktree management.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrDirtyWorktree is returned when a worktree has uncommitted changes.
var ErrDirtyWorktree = errors.New("worktree has uncommitted changes")

// RunGit runs a git command in the given working directory and returns trimmed stdout.
func RunGit(workingDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workingDir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// WorktreeAdd creates a new git worktree.
// If createBranch is true, passes -b <branch> to create a new branch.
// If createBranch is false, checks out the existing branch at the worktree path.
func WorktreeAdd(repoPath, worktreePath, base, branch string, createBranch bool) error {
	args := []string{"worktree", "add"}
	if createBranch {
		// Use -c to prevent the new branch from automatically tracking the
		// remote base ref (e.g. origin/main) as its upstream. Users can set
		// the upstream manually if needed.
		args = []string{"-c", "branch.autoSetupMerge=false", "worktree", "add", "-b", branch}
	}
	args = append(args, worktreePath)
	if base != "" {
		args = append(args, base)
	}
	if !createBranch && branch != "" {
		// Checking out existing branch: worktree add <path> <branch>
		// base was already appended as the branch name in this case
		// Reset args and use the branch as the final positional arg
		args = []string{"worktree", "add", worktreePath, branch}
	}
	_, err := RunGit(repoPath, args...)
	if err != nil {
		return fmt.Errorf("worktree add: %w", err)
	}
	return nil
}

// WorktreeAddDetached creates a new git worktree in detached HEAD state.
func WorktreeAddDetached(repoPath, worktreePath, commitish string) error {
	_, err := RunGit(repoPath, "worktree", "add", "--detach", worktreePath, commitish)
	if err != nil {
		return fmt.Errorf("worktree add detached: %w", err)
	}
	return nil
}

// HeadSHA returns the full 40-character SHA of HEAD in the given directory.
func HeadSHA(dir string) (string, error) {
	out, err := RunGit(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("head sha: %w", err)
	}
	return out, nil
}

// WorktreeRemove removes a git worktree from the repository.
func WorktreeRemove(repoPath, worktreePath string) error {
	_, err := RunGit(repoPath, "worktree", "remove", worktreePath)
	if err != nil {
		return fmt.Errorf("worktree remove: %w", err)
	}
	return nil
}

// WorktreeList returns the list of worktrees for the repository in porcelain format lines.
func WorktreeList(repoPath string) (string, error) {
	out, err := RunGit(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("worktree list: %w", err)
	}
	return out, nil
}

// WorktreeForBranch returns the worktree path that has the given branch checked out,
// or "" if the branch is not checked out in any worktree. It parses the porcelain
// output of `git worktree list`.
func WorktreeForBranch(repoPath, branch string) (string, error) {
	out, err := WorktreeList(repoPath)
	if err != nil {
		return "", err
	}
	target := "branch refs/heads/" + branch
	var currentWorktree string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
		} else if line == target {
			return currentWorktree, nil
		} else if line == "" {
			currentWorktree = ""
		}
	}
	return "", nil
}

// BranchCheckedOut returns true if the given branch is already checked out in any
// worktree of the repository. This is useful for pre-validation before --checkout,
// since git does not allow the same branch to be checked out in multiple worktrees.
func BranchCheckedOut(repoPath, branch string) (bool, error) {
	path, err := WorktreeForBranch(repoPath, branch)
	if err != nil {
		return false, err
	}
	return path != "", nil
}

// StatusPorcelain returns whether a worktree is clean and any status lines.
func StatusPorcelain(worktreeDir string) (clean bool, lines []string, err error) {
	out, err := RunGit(worktreeDir, "status", "--porcelain")
	if err != nil {
		return false, nil, fmt.Errorf("status porcelain: %w", err)
	}
	if out == "" {
		return true, nil, nil
	}
	lines = strings.Split(out, "\n")
	return false, lines, nil
}

// RefExists returns true if the given ref (branch, tag, commit, remote ref) exists in the repo.
func RefExists(repoPath, ref string) (bool, error) {
	_, err := RunGit(repoPath, "rev-parse", "--verify", ref)
	if err != nil {
		// exit code 128 means ref doesn't exist
		return false, nil
	}
	return true, nil
}

// BranchExists returns true if a local branch with the given name exists.
func BranchExists(repoPath, branch string) (bool, error) {
	_, err := RunGit(repoPath, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CurrentBranch returns the name of the currently checked-out branch in dir.
// Returns an error if in detached HEAD state.
func CurrentBranch(dir string) (string, error) {
	out, err := RunGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("current branch: %w", err)
	}
	if out == "HEAD" {
		return "", fmt.Errorf("current branch: detached HEAD state")
	}
	return out, nil
}

// AvailableBranchName finds an unused branch name starting from baseName.
// Strategy: baseName → baseName-YYYY-MM-DD → baseName-YYYY-MM-DD-1, -2, …
func AvailableBranchName(repoPath, baseName string, now time.Time) (string, error) {
	// Try baseName first
	exists, err := BranchExists(repoPath, baseName)
	if err != nil {
		return "", err
	}
	if !exists {
		return baseName, nil
	}

	// Try baseName-YYYY-MM-DD
	dated := baseName + "-" + now.Format("2006-01-02")
	exists, err = BranchExists(repoPath, dated)
	if err != nil {
		return "", err
	}
	if !exists {
		return dated, nil
	}

	// Try baseName-YYYY-MM-DD-N
	for i := 1; i <= 999; i++ {
		candidate := dated + "-" + strconv.Itoa(i)
		exists, err = BranchExists(repoPath, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not find available branch name starting from %q", baseName)
}

// Remotes returns the list of remote names configured for the repository.
func Remotes(repoPath string) ([]string, error) {
	out, err := RunGit(repoPath, "remote")
	if err != nil {
		return nil, fmt.Errorf("remotes: %w", err)
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// RemoteForRef returns the remote name if ref is a remote-tracking ref (e.g. "origin/main" → "origin"),
// or "" if it is a local ref. The check is done by matching configured remote names as prefixes.
func RemoteForRef(repoPath, ref string) (string, error) {
	remotes, err := Remotes(repoPath)
	if err != nil {
		return "", err
	}
	for _, remote := range remotes {
		if strings.HasPrefix(ref, remote+"/") {
			return remote, nil
		}
	}
	return "", nil
}

// Fetch fetches all refs from the given remote.
func Fetch(repoPath, remote string) error {
	_, err := RunGit(repoPath, "fetch", remote)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", remote, err)
	}
	return nil
}

// FetchRef fetches a single ref from the given remote. This is faster than
// Fetch for large repos because only the named ref (and its ancestors) are
// transferred instead of every ref on the remote.
func FetchRef(repoPath, remote, ref string) error {
	_, err := RunGit(repoPath, "fetch", remote, ref)
	if err != nil {
		return fmt.Errorf("fetch %s %s: %w", remote, ref, err)
	}
	return nil
}

// HasUnpushedCommits returns true if the given branch has commits that have not
// been pushed to any remote. Returns false (no check performed) if the repo has
// no remotes configured.
func HasUnpushedCommits(repoPath, branch string) (bool, error) {
	remotes, err := Remotes(repoPath)
	if err != nil {
		return false, err
	}
	if len(remotes) == 0 {
		return false, nil
	}
	out, err := RunGit(repoPath, "log", branch, "--not", "--remotes", "--oneline")
	if err != nil {
		return false, fmt.Errorf("check unpushed commits: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// BranchNameFromRef extracts a local branch name from a ref string.
// If the ref looks like a remote-tracking ref (e.g. "origin/feature"), it strips the
// remote prefix. Otherwise the ref is returned as-is.
// Examples: "origin/feature" → "feature", "upstream/main" → "main", "my-branch" → "my-branch".
func BranchNameFromRef(repoPath, ref string) (string, error) {
	remote, err := RemoteForRef(repoPath, ref)
	if err != nil {
		return "", fmt.Errorf("branch name from ref: %w", err)
	}
	if remote != "" {
		return strings.TrimPrefix(ref, remote+"/"), nil
	}
	return ref, nil
}

// MinVersionCheck verifies the installed git is at least version 2.5 (first worktree support).
func MinVersionCheck() error {
	out, err := RunGit(".", "version")
	if err != nil {
		return fmt.Errorf("git version check: %w", err)
	}
	// output: "git version 2.39.2"
	parts := strings.Fields(out)
	if len(parts) < 3 {
		return fmt.Errorf("unexpected git version output: %q", out)
	}
	versionStr := parts[2]
	segments := strings.Split(versionStr, ".")
	if len(segments) < 2 {
		return fmt.Errorf("cannot parse git version: %q", versionStr)
	}
	major, err := strconv.Atoi(segments[0])
	if err != nil {
		return fmt.Errorf("cannot parse git major version: %w", err)
	}
	minor, err := strconv.Atoi(segments[1])
	if err != nil {
		return fmt.Errorf("cannot parse git minor version: %w", err)
	}
	if major < 2 || (major == 2 && minor < 5) {
		return fmt.Errorf("git 2.5+ required, found %s", versionStr)
	}
	return nil
}
