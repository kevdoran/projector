package git_test

import (
	"os"
	"path/filepath"
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

func TestMinVersionCheck(t *testing.T) {
	if err := git.MinVersionCheck(); err != nil {
		t.Fatalf("MinVersionCheck: %v", err)
	}
}
