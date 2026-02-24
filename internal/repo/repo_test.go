package repo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kevdoran/projector/internal/repo"
)

// makeGitRepo creates a directory with a .git subdirectory to simulate a git repo.
func makeGitRepo(t *testing.T, parent, name string) string {
	t.Helper()
	repoPath := filepath.Join(parent, name)
	gitPath := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitPath, 0755); err != nil {
		t.Fatalf("makeGitRepo: %v", err)
	}
	return repoPath
}

func TestDiscover_FindsGitRepos(t *testing.T) {
	searchDir := t.TempDir()
	makeGitRepo(t, searchDir, "repo-a")
	makeGitRepo(t, searchDir, "repo-b")
	// Non-repo directory should be ignored
	if err := os.MkdirAll(filepath.Join(searchDir, "not-a-repo"), 0755); err != nil {
		t.Fatal(err)
	}
	// Worktree .git file (not a directory) should be ignored
	worktreeDir := filepath.Join(searchDir, "worktree-dir")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: /fake"), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := repo.Discover([]string{searchDir})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}
	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["repo-a"] || !names["repo-b"] {
		t.Errorf("expected repo-a and repo-b, got %v", repos)
	}
}

func TestDiscover_MultipleSearchDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	makeGitRepo(t, dir1, "repo-1")
	makeGitRepo(t, dir2, "repo-2")

	repos, err := repo.Discover([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
}

func TestDiscover_MissingDirIgnored(t *testing.T) {
	repos, err := repo.Discover([]string{"/nonexistent/path/xyz"})
	if err != nil {
		t.Fatalf("expected no error for missing dir, got: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

func TestResolveRepos_ByName(t *testing.T) {
	searchDir := t.TempDir()
	makeGitRepo(t, searchDir, "my-repo")

	repos, err := repo.ResolveRepos([]string{"my-repo"}, []string{searchDir})
	if err != nil {
		t.Fatalf("ResolveRepos: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "my-repo" {
		t.Errorf("unexpected repos: %v", repos)
	}
}

func TestResolveRepos_ByAbsPath(t *testing.T) {
	dir := t.TempDir()
	repoPath := makeGitRepo(t, dir, "abs-repo")

	repos, err := repo.ResolveRepos([]string{repoPath}, nil)
	if err != nil {
		t.Fatalf("ResolveRepos: %v", err)
	}
	if len(repos) != 1 || repos[0].Path != repoPath {
		t.Errorf("unexpected repos: %v", repos)
	}
}

func TestResolveRepos_UnknownName(t *testing.T) {
	_, err := repo.ResolveRepos([]string{"nonexistent"}, []string{t.TempDir()})
	if err == nil {
		t.Fatal("expected error for unknown repo name")
	}
}

func TestResolveRepos_InvalidAbsPath(t *testing.T) {
	dir := t.TempDir() // not a git repo (no .git dir)
	_, err := repo.ResolveRepos([]string{dir}, nil)
	if err == nil {
		t.Fatal("expected error for non-git abs path")
	}
}
