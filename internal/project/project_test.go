package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

// createTestRepo initialises a real git repo with an initial commit.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := git.RunGit(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")
	return dir
}

func TestSave_And_Load_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	cfg := &project.ProjectConfig{
		Project: project.ProjectMeta{
			Name:      "my-project",
			CreatedAt: now,
			Status:    project.StatusActive,
		},
	}
	if err := project.Save(cfg, dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := project.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Project.Name != "my-project" {
		t.Errorf("Name: got %q, want %q", loaded.Project.Name, "my-project")
	}
	if loaded.Project.Status != project.StatusActive {
		t.Errorf("Status: got %q", loaded.Project.Status)
	}
	if !loaded.Project.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", loaded.Project.CreatedAt, now)
	}
	if loaded.ConfigVersion != project.CurrentConfigVersion {
		t.Errorf("ConfigVersion: got %d, want %d", loaded.ConfigVersion, project.CurrentConfigVersion)
	}
}

func TestLoad_ErrConfigVersionTooNew(t *testing.T) {
	dir := t.TempDir()
	content := "config-version = 9999\n\n[project]\nname = \"foo\"\nstatus = \"active\"\ncreated-at = 2026-01-01T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, ".projector.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := project.Load(dir)
	if !errors.Is(err, project.ErrConfigVersionTooNew) {
		t.Fatalf("expected ErrConfigVersionTooNew, got %v", err)
	}
}

func TestLoad_ErrNotFound(t *testing.T) {
	_, err := project.Load(t.TempDir())
	if !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListAll_SortOrder(t *testing.T) {
	projectsDir := t.TempDir()
	now := time.Now().UTC()

	// Create: archived "zoo", active "alpha", active "beta"
	for _, name := range []string{"zoo", "alpha", "beta"} {
		status := project.StatusActive
		if name == "zoo" {
			status = project.StatusArchived
		}
		dir := filepath.Join(projectsDir, name)
		cfg := &project.ProjectConfig{
			Project: project.ProjectMeta{Name: name, CreatedAt: now, Status: status},
		}
		if err := project.Save(cfg, dir); err != nil {
			t.Fatal(err)
		}
	}
	// Create a directory without .projector.toml — should be skipped
	if err := os.MkdirAll(filepath.Join(projectsDir, "not-a-project"), 0755); err != nil {
		t.Fatal(err)
	}

	projects, err := project.ListAll(projectsDir)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	// active first (alpha, beta), then archived (zoo)
	want := []string{"alpha", "beta", "zoo"}
	for i, p := range projects {
		if p.Project.Name != want[i] {
			t.Errorf("index %d: got %q, want %q", i, p.Project.Name, want[i])
		}
	}
}

func TestListAll_EmptyDir(t *testing.T) {
	projects, err := project.ListAll(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestFindProjectDir_Found(t *testing.T) {
	projectsDir := t.TempDir()
	dir := filepath.Join(projectsDir, "myproject")
	cfg := &project.ProjectConfig{
		Project: project.ProjectMeta{Name: "myproject", CreatedAt: time.Now(), Status: project.StatusActive},
	}
	if err := project.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}

	// Find from project dir itself
	found, err := project.FindProjectDir(dir)
	if err != nil {
		t.Fatalf("FindProjectDir: %v", err)
	}
	if found != dir {
		t.Errorf("got %q, want %q", found, dir)
	}

	// Find from a subdirectory of the project
	subDir := filepath.Join(dir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	found, err = project.FindProjectDir(subDir)
	if err != nil {
		t.Fatalf("FindProjectDir from subdir: %v", err)
	}
	if found != dir {
		t.Errorf("from subdir: got %q, want %q", found, dir)
	}
}

func TestFindProjectDir_NotFound(t *testing.T) {
	_, err := project.FindProjectDir(t.TempDir())
	if !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"foo", "my-project", "v2.1", "project_name", "ABC123"}
	for _, name := range valid {
		if err := project.ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) unexpected error: %v", name, err)
		}
	}
	invalid := []string{"", "has space", "with/slash", "with\\backslash", "with:colon"}
	for _, name := range invalid {
		if err := project.ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) expected error", name)
		}
	}
}

func TestDiscoverWorktrees(t *testing.T) {
	repo := createTestRepo(t)
	projectDir := t.TempDir()

	// Add a worktree into the project dir
	worktreePath := filepath.Join(projectDir, "my-repo+myproject")
	if err := git.WorktreeAdd(repo, worktreePath, "HEAD", "myproject", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	worktrees, err := project.DiscoverWorktrees(projectDir)
	if err != nil {
		t.Fatalf("DiscoverWorktrees: %v", err)
	}
	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}
	wt := worktrees[0]
	// Use EvalSymlinks to handle macOS /var → /private/var symlink.
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	if resolve(wt.WorktreePath) != resolve(worktreePath) {
		t.Errorf("WorktreePath: got %q, want %q", wt.WorktreePath, worktreePath)
	}
	if resolve(wt.RepoPath) != resolve(repo) {
		t.Errorf("RepoPath: got %q, want %q", wt.RepoPath, repo)
	}
	if wt.Branch != "myproject" {
		t.Errorf("Branch: got %q, want %q", wt.Branch, "myproject")
	}
}

func TestDiscoverWorktrees_NoWorktrees(t *testing.T) {
	projectDir := t.TempDir()
	// Add a regular subdir (not a worktree)
	if err := os.MkdirAll(filepath.Join(projectDir, "notes"), 0755); err != nil {
		t.Fatal(err)
	}

	worktrees, err := project.DiscoverWorktrees(projectDir)
	if err != nil {
		t.Fatalf("DiscoverWorktrees: %v", err)
	}
	if len(worktrees) != 0 {
		t.Fatalf("expected 0 worktrees, got %d", len(worktrees))
	}
}
