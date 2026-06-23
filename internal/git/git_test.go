package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevdoran/projector/internal/git"
)

// createTestRepo initialises a git repo with an initial commit in a fresh temp
// directory and returns its path.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	makeRepoAt(t, dir)
	return dir
}

// makeRepoAt initialises a git repo with an initial commit at the given path,
// creating the directory if needed. Useful when the repo must live at a
// specific location (e.g. under a parent directory that will be moved).
func makeRepoAt(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

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

func TestWorktreeRepair(t *testing.T) {
	// This exercises the git wrapper directly against a "repo and worktree moved
	// together" scenario: a single parent directory holds BOTH the repo and the
	// worktree, and renaming the parent breaks both pointers. The test then passes
	// the post-move repo path straight to WorktreeRepair.
	//
	// Note this is NOT the path the `pj project repair` command takes — the command
	// discovers the repo from the worktree's still-valid .git file and never sees a
	// repo that has itself moved. For a faithful reproduction of pj's common layout
	// (external repo, only the project dir renamed) see
	// TestWorktreeRepair_RenamedProjectDir below.
	//
	// Simulate a project that is moved/renamed: a parent directory containing both
	// the repo and a worktree. Renaming the parent leaves both git pointers stale
	// (the worktree's .git file points at the old repo path, and the repo's
	// administrative gitdir points at the old worktree path).
	root := t.TempDir()
	oldDir := filepath.Join(root, "project-old")
	if err := os.Mkdir(oldDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	repo := filepath.Join(oldDir, "repo")
	makeRepoAt(t, repo)

	wt := filepath.Join(oldDir, "wt")
	if err := git.WorktreeAdd(repo, wt, "HEAD", "repair-branch", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// Sanity check: worktree is healthy before the move.
	if _, err := git.RunGit(wt, "status"); err != nil {
		t.Fatalf("worktree should be healthy before move: %v", err)
	}

	// Rename the whole project directory.
	newDir := filepath.Join(root, "project-new")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatalf("rename project dir: %v", err)
	}
	newRepo := filepath.Join(newDir, "repo")
	newWt := filepath.Join(newDir, "wt")

	// The worktree is now broken: its .git file still points at the old repo path.
	if _, err := git.RunGit(newWt, "status"); err == nil {
		t.Fatal("expected git status to fail in moved worktree before repair")
	}

	// Repair from the (moved) repo, passing the new worktree path so git can find it.
	if err := git.WorktreeRepair(newRepo, newWt); err != nil {
		t.Fatalf("WorktreeRepair: %v", err)
	}

	// After repair the worktree resolves again.
	if _, err := git.RunGit(newWt, "status"); err != nil {
		t.Fatalf("worktree should be healthy after repair: %v", err)
	}

	// git worktree list should reference the new path, not the old one.
	out, err := git.WorktreeList(newRepo)
	if err != nil {
		t.Fatalf("WorktreeList: %v", err)
	}
	if !strings.Contains(out, newWt) {
		t.Fatalf("worktree list does not reference new path %q:\n%s", newWt, out)
	}
	if strings.Contains(out, wt) {
		t.Fatalf("worktree list still references old path %q:\n%s", wt, out)
	}
}

// TestWorktreeRepair_RenamedProjectDir reproduces pj's common layout and the
// exact breakage the `pj project repair` command fixes:
//
//   - The git repo lives EXTERNALLY (in a repo-search-dir), outside the project dir.
//   - A worktree is created under a separate "project dir".
//   - Only the PROJECT DIR is renamed; the external repo stays put.
//
// Because the external repo did not move, the worktree's own .git file (an
// absolute path to the external repo) stays valid, so `git -C <worktree> status`
// keeps working. But the repo's administrative pointer
// (.git/worktrees/<id>/gitdir) goes stale, so `git worktree list` from the repo
// marks the worktree as `prunable` and still points at the OLD path.
//
// `git worktree repair <newWorktreePath>` run from the external repo — exactly
// what the command does (anchored at wt.RepoPath with the new worktree path as an
// arg) — fixes the stale admin pointer.
func TestWorktreeRepair_RenamedProjectDir(t *testing.T) {
	// External repo, outside the project dir.
	repo := createTestRepo(t)

	// Project dir holding the worktree, in a separate location.
	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	wt := filepath.Join(projectDir, "myrepo")
	if err := git.WorktreeAdd(repo, wt, "HEAD", "feature-branch", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// Sanity: worktree healthy and listed at its current path before the move.
	if _, err := git.RunGit(wt, "status"); err != nil {
		t.Fatalf("worktree should be healthy before move: %v", err)
	}

	// Rename ONLY the project dir; the external repo stays put.
	projectDirRenamed := filepath.Join(filepath.Dir(projectDir), "project-renamed")
	if err := os.Rename(projectDir, projectDirRenamed); err != nil {
		t.Fatalf("rename project dir: %v", err)
	}
	newWt := filepath.Join(projectDirRenamed, "myrepo")

	// The worktree's own .git file still points at the (unmoved) external repo, so
	// status from the new path keeps working — this is what masks the problem from
	// the worktree's perspective.
	if _, err := git.RunGit(newWt, "status"); err != nil {
		t.Fatalf("worktree status from new path should still work (external repo did not move): %v", err)
	}

	// Assert the breakage: the repo's admin pointer is stale, so worktree list
	// marks the worktree prunable and still references the OLD path.
	before, err := git.RunGit(repo, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("worktree list (before repair): %v", err)
	}
	if !strings.Contains(before, "prunable") {
		t.Fatalf("expected stale worktree to be marked prunable before repair:\n%s", before)
	}
	if !strings.Contains(before, wt) {
		t.Fatalf("expected worktree list to still reference old path %q before repair:\n%s", wt, before)
	}
	if strings.Contains(before, newWt) {
		t.Fatalf("did not expect worktree list to reference new path %q before repair:\n%s", newWt, before)
	}

	// Repair from the external repo, passing the new worktree path — exactly what
	// the command does.
	if err := git.WorktreeRepair(repo, newWt); err != nil {
		t.Fatalf("WorktreeRepair: %v", err)
	}

	// Assert the fix: the admin pointer now references the new path and is no
	// longer prunable.
	after, err := git.RunGit(repo, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("worktree list (after repair): %v", err)
	}
	if strings.Contains(after, "prunable") {
		t.Fatalf("worktree should not be prunable after repair:\n%s", after)
	}
	if !strings.Contains(after, newWt) {
		t.Fatalf("worktree list should reference new path %q after repair:\n%s", newWt, after)
	}
	if strings.Contains(after, wt) {
		t.Fatalf("worktree list should not reference old path %q after repair:\n%s", wt, after)
	}

	// And the worktree itself is still healthy.
	if _, err := git.RunGit(newWt, "status"); err != nil {
		t.Fatalf("worktree should be healthy after repair: %v", err)
	}
}

func TestWorktreeRepair_NoWorktrees(t *testing.T) {
	repo := createTestRepo(t)
	// Repairing a repo with no extra worktrees should be a no-op, not an error.
	if err := git.WorktreeRepair(repo); err != nil {
		t.Fatalf("WorktreeRepair on repo with no worktrees: %v", err)
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

func TestDefaultRemote(t *testing.T) {
	t.Run("no remotes", func(t *testing.T) {
		repo := createTestRepo(t)
		got, err := git.DefaultRemote(repo)
		if err != nil {
			t.Fatalf("DefaultRemote: %v", err)
		}
		if got != "" {
			t.Fatalf("expected no default remote, got %q", got)
		}
	})

	t.Run("origin via clone", func(t *testing.T) {
		upstream := createTestRepo(t)
		cloneDir := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, cloneDir); err != nil {
			t.Fatalf("clone: %v", err)
		}
		got, err := git.DefaultRemote(cloneDir)
		if err != nil {
			t.Fatalf("DefaultRemote: %v", err)
		}
		if got != "origin" {
			t.Fatalf("expected origin, got %q", got)
		}
	})

	t.Run("sole non-origin remote", func(t *testing.T) {
		upstream := createTestRepo(t)
		repo := createTestRepo(t)
		if _, err := git.RunGit(repo, "remote", "add", "upstream", upstream); err != nil {
			t.Fatalf("remote add: %v", err)
		}
		got, err := git.DefaultRemote(repo)
		if err != nil {
			t.Fatalf("DefaultRemote: %v", err)
		}
		if got != "upstream" {
			t.Fatalf("expected upstream, got %q", got)
		}
	})

	t.Run("multiple remotes without origin", func(t *testing.T) {
		a := createTestRepo(t)
		b := createTestRepo(t)
		repo := createTestRepo(t)
		if _, err := git.RunGit(repo, "remote", "add", "fork-a", a); err != nil {
			t.Fatalf("remote add: %v", err)
		}
		if _, err := git.RunGit(repo, "remote", "add", "fork-b", b); err != nil {
			t.Fatalf("remote add: %v", err)
		}
		got, err := git.DefaultRemote(repo)
		if err != nil {
			t.Fatalf("DefaultRemote: %v", err)
		}
		if got != "" {
			t.Fatalf("expected no default remote with multiple non-origin remotes, got %q", got)
		}
	})

	t.Run("origin preferred over others", func(t *testing.T) {
		upstream := createTestRepo(t)
		cloneDir := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, cloneDir); err != nil {
			t.Fatalf("clone: %v", err)
		}
		other := createTestRepo(t)
		if _, err := git.RunGit(cloneDir, "remote", "add", "upstream", other); err != nil {
			t.Fatalf("remote add: %v", err)
		}
		got, err := git.DefaultRemote(cloneDir)
		if err != nil {
			t.Fatalf("DefaultRemote: %v", err)
		}
		if got != "origin" {
			t.Fatalf("expected origin to be preferred, got %q", got)
		}
	})
}
