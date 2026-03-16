package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevdoran/projector/internal/git"
)

// createTestRepo initialises a git repo with an initial commit and returns its path.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		out, err := git.RunGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	// Create an initial commit so HEAD exists
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestRunGit_Success(t *testing.T) {
	dir := createTestRepo(t)
	out, err := git.RunGit(dir, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRunGit_Error(t *testing.T) {
	dir := t.TempDir()
	_, err := git.RunGit(dir, "status")
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

func TestWorktreeAdd_And_Remove(t *testing.T) {
	repo := createTestRepo(t)
	worktreePath := filepath.Join(t.TempDir(), "my-worktree")

	// Add worktree with new branch
	if err := git.WorktreeAdd(repo, worktreePath, "HEAD", "feature-branch", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// Verify worktree directory was created
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	// Remove worktree
	if err := git.WorktreeRemove(repo, worktreePath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}

	// Verify directory was removed by git
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatal("expected worktree dir to be removed")
	}
}

func TestWorktreeAdd_ExistingBranch(t *testing.T) {
	repo := createTestRepo(t)

	// First create the branch
	wt1 := filepath.Join(t.TempDir(), "wt1")
	if err := git.WorktreeAdd(repo, wt1, "HEAD", "existing-branch", true); err != nil {
		t.Fatalf("WorktreeAdd (create): %v", err)
	}
	// Remove so we can re-add without branch
	if err := git.WorktreeRemove(repo, wt1); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}

	// Re-add without creating branch
	wt2 := filepath.Join(t.TempDir(), "wt2")
	if err := git.WorktreeAdd(repo, wt2, "", "existing-branch", false); err != nil {
		t.Fatalf("WorktreeAdd (existing): %v", err)
	}
	if _, err := os.Stat(wt2); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}
	if err := git.WorktreeRemove(repo, wt2); err != nil {
		t.Fatalf("cleanup WorktreeRemove: %v", err)
	}
}

func TestStatusPorcelain_Clean(t *testing.T) {
	repo := createTestRepo(t)
	clean, lines, err := git.StatusPorcelain(repo)
	if err != nil {
		t.Fatalf("StatusPorcelain: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean repo, got lines: %v", lines)
	}
}

func TestStatusPorcelain_Dirty(t *testing.T) {
	repo := createTestRepo(t)

	// Create an untracked file
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}

	clean, lines, err := git.StatusPorcelain(repo)
	if err != nil {
		t.Fatalf("StatusPorcelain: %v", err)
	}
	if clean {
		t.Fatal("expected dirty repo")
	}
	if len(lines) == 0 {
		t.Fatal("expected status lines")
	}
}

func TestRefExists(t *testing.T) {
	repo := createTestRepo(t)

	// HEAD should exist
	exists, err := git.RefExists(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected HEAD to exist")
	}

	// Non-existent ref
	exists, err = git.RefExists(repo, "refs/heads/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected nonexistent ref to not exist")
	}
}

func TestBranchExists(t *testing.T) {
	repo := createTestRepo(t)

	// The initial branch (main or master) should exist
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := git.BranchExists(repo, branch)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("expected branch %q to exist", branch)
	}

	exists, err = git.BranchExists(repo, "no-such-branch")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no-such-branch to not exist")
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := createTestRepo(t)
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}

func TestAvailableBranchName_Fresh(t *testing.T) {
	repo := createTestRepo(t)
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	name, err := git.AvailableBranchName(repo, "feature", now)
	if err != nil {
		t.Fatalf("AvailableBranchName: %v", err)
	}
	if name != "feature" {
		t.Errorf("expected 'feature', got %q", name)
	}
}

func TestAvailableBranchName_Collision(t *testing.T) {
	repo := createTestRepo(t)
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	// Create branch "feature" to force collision
	wt := filepath.Join(t.TempDir(), "wt-feature")
	if err := git.WorktreeAdd(repo, wt, "HEAD", "feature", true); err != nil {
		t.Fatalf("setup WorktreeAdd: %v", err)
	}

	name, err := git.AvailableBranchName(repo, "feature", now)
	if err != nil {
		t.Fatalf("AvailableBranchName: %v", err)
	}
	expected := "feature-2026-02-23"
	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestAvailableBranchName_DoubleCollision(t *testing.T) {
	repo := createTestRepo(t)
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	// Create "feature" and "feature-2026-02-23"
	wt1 := filepath.Join(t.TempDir(), "wt1")
	if err := git.WorktreeAdd(repo, wt1, "HEAD", "feature", true); err != nil {
		t.Fatalf("setup: %v", err)
	}
	wt2 := filepath.Join(t.TempDir(), "wt2")
	if err := git.WorktreeAdd(repo, wt2, "HEAD", "feature-2026-02-23", true); err != nil {
		t.Fatalf("setup: %v", err)
	}

	name, err := git.AvailableBranchName(repo, "feature", now)
	if err != nil {
		t.Fatalf("AvailableBranchName: %v", err)
	}
	expected := "feature-2026-02-23-1"
	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestWorktreeAdd_NoUpstreamTracking(t *testing.T) {
	// Create an "upstream" repo and clone it so we have remote-tracking refs.
	upstream := createTestRepo(t)
	cloneDir := filepath.Join(t.TempDir(), "clone")
	if _, err := git.RunGit(t.TempDir(), "clone", upstream, cloneDir); err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Determine the default branch name in the clone
	defaultBranch, err := git.CurrentBranch(cloneDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	// Create a worktree branching from origin/<default>
	wtPath := filepath.Join(t.TempDir(), "wt-no-track")
	base := "origin/" + defaultBranch
	if err := git.WorktreeAdd(cloneDir, wtPath, base, "my-feature", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// Verify no upstream is set on the new branch
	_, err = git.RunGit(cloneDir, "config", "branch.my-feature.remote")
	if err == nil {
		t.Fatal("expected branch.my-feature.remote to be unset, but it was configured")
	}
}

func TestWorktreeAddDetached(t *testing.T) {
	repo := createTestRepo(t)
	worktreePath := filepath.Join(t.TempDir(), "detached-wt")

	if err := git.WorktreeAddDetached(repo, worktreePath, "HEAD"); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}

	// Verify worktree directory was created
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	// Verify it's in detached HEAD state
	branch, err := git.CurrentBranch(worktreePath)
	if err == nil {
		t.Fatalf("expected detached HEAD error, got branch %q", branch)
	}

	// Clean up
	if err := git.WorktreeRemove(repo, worktreePath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
}

func TestHeadSHA(t *testing.T) {
	repo := createTestRepo(t)
	sha, err := git.HeadSHA(repo)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if len(sha) != 40 {
		t.Fatalf("expected 40-char SHA, got %q (len %d)", sha, len(sha))
	}
}

func TestWorktreeForBranch(t *testing.T) {
	repo := createTestRepo(t)

	// Resolve symlinks so comparisons work on macOS (/var → /private/var).
	repoReal, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	// The default branch should be checked out in the main worktree (the repo dir itself).
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	wtPath, err := git.WorktreeForBranch(repo, branch)
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if wtPath != repoReal {
		t.Fatalf("expected worktree path %q, got %q", repoReal, wtPath)
	}

	// A branch that doesn't exist should return empty string.
	wtPath, err = git.WorktreeForBranch(repo, "nonexistent-branch")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if wtPath != "" {
		t.Fatalf("expected empty path for nonexistent branch, got %q", wtPath)
	}

	// Create a new branch in a worktree and verify the path is returned.
	newWt := filepath.Join(t.TempDir(), "wt-for-branch")
	newWtReal, err := filepath.EvalSymlinks(filepath.Dir(newWt))
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	newWtReal = filepath.Join(newWtReal, "wt-for-branch")

	if err := git.WorktreeAdd(repo, newWt, "HEAD", "wt-branch-test", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	wtPath, err = git.WorktreeForBranch(repo, "wt-branch-test")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if wtPath != newWtReal {
		t.Fatalf("expected worktree path %q, got %q", newWtReal, wtPath)
	}

	// Remove the worktree — should return empty string.
	if err := git.WorktreeRemove(repo, newWt); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	wtPath, err = git.WorktreeForBranch(repo, "wt-branch-test")
	if err != nil {
		t.Fatalf("WorktreeForBranch: %v", err)
	}
	if wtPath != "" {
		t.Fatalf("expected empty path after removal, got %q", wtPath)
	}
}

func TestBranchCheckedOut(t *testing.T) {
	repo := createTestRepo(t)

	// The default branch should be checked out in the main worktree.
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	inUse, err := git.BranchCheckedOut(repo, branch)
	if err != nil {
		t.Fatalf("BranchCheckedOut: %v", err)
	}
	if !inUse {
		t.Fatalf("expected default branch %q to be checked out", branch)
	}

	// A branch that doesn't exist should not be checked out.
	inUse, err = git.BranchCheckedOut(repo, "nonexistent-branch")
	if err != nil {
		t.Fatalf("BranchCheckedOut: %v", err)
	}
	if inUse {
		t.Fatal("expected nonexistent branch to not be checked out")
	}

	// Create a new branch in a worktree, then verify it's detected.
	wtPath := filepath.Join(t.TempDir(), "wt-checked-out")
	if err := git.WorktreeAdd(repo, wtPath, "HEAD", "feature-in-use", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	inUse, err = git.BranchCheckedOut(repo, "feature-in-use")
	if err != nil {
		t.Fatalf("BranchCheckedOut: %v", err)
	}
	if !inUse {
		t.Fatal("expected feature-in-use to be checked out")
	}

	// Remove the worktree — branch should no longer be checked out.
	if err := git.WorktreeRemove(repo, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	inUse, err = git.BranchCheckedOut(repo, "feature-in-use")
	if err != nil {
		t.Fatalf("BranchCheckedOut: %v", err)
	}
	if inUse {
		t.Fatal("expected feature-in-use to not be checked out after worktree removal")
	}
}

func TestBranchNameFromRef(t *testing.T) {
	// Create an upstream repo and clone it so we have remote-tracking refs.
	upstream := createTestRepo(t)
	cloneDir := filepath.Join(t.TempDir(), "clone")
	if _, err := git.RunGit(t.TempDir(), "clone", upstream, cloneDir); err != nil {
		t.Fatalf("clone: %v", err)
	}

	defaultBranch, err := git.CurrentBranch(cloneDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{"remote ref", "origin/" + defaultBranch, defaultBranch},
		{"local ref", "my-branch", "my-branch"},
		{"plain name", "feature-x", "feature-x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := git.BranchNameFromRef(cloneDir, tc.ref)
			if err != nil {
				t.Fatalf("BranchNameFromRef(%q): %v", tc.ref, err)
			}
			if got != tc.expected {
				t.Errorf("BranchNameFromRef(%q) = %q, want %q", tc.ref, got, tc.expected)
			}
		})
	}
}

func TestMinVersionCheck(t *testing.T) {
	if err := git.MinVersionCheck(); err != nil {
		t.Fatalf("MinVersionCheck: %v", err)
	}
}

func TestFetchRef(t *testing.T) {
	// Create an "upstream" repo and clone it.
	upstream := createTestRepo(t)
	cloneDir := filepath.Join(t.TempDir(), "clone")
	if _, err := git.RunGit(t.TempDir(), "clone", upstream, cloneDir); err != nil {
		t.Fatalf("clone: %v", err)
	}

	defaultBranch, err := git.CurrentBranch(cloneDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	// Add a new commit to the upstream repo so the clone is behind.
	f := filepath.Join(upstream, "new-file.txt")
	if err := os.WriteFile(f, []byte("new content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.RunGit(upstream, "add", "new-file.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := git.RunGit(upstream, "commit", "-m", "second commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Record the new upstream HEAD.
	upstreamSHA, err := git.HeadSHA(upstream)
	if err != nil {
		t.Fatalf("HeadSHA upstream: %v", err)
	}

	// FetchRef should bring in only the named branch.
	if err := git.FetchRef(cloneDir, "origin", defaultBranch); err != nil {
		t.Fatalf("FetchRef: %v", err)
	}

	// Verify origin/<default> now points at the new commit.
	out, err := git.RunGit(cloneDir, "rev-parse", "origin/"+defaultBranch)
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	if strings.TrimSpace(out) != upstreamSHA {
		t.Fatalf("expected origin/%s to be %s, got %s", defaultBranch, upstreamSHA, strings.TrimSpace(out))
	}
}
