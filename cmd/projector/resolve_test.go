package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kevdoran/projector/internal/git"
)

// initTestRepo initialises a git repo with an initial commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if out, err := git.RunGit(dir, args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestResolveBaseRef(t *testing.T) {
	t.Run("local ref used as-is", func(t *testing.T) {
		repo := initTestRepo(t)
		if _, err := git.RunGit(repo, "branch", "feature-local"); err != nil {
			t.Fatalf("branch: %v", err)
		}

		resolved, found, err := resolveBaseRef("repo", repo, "feature-local")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if resolved != "feature-local" {
			t.Fatalf("expected feature-local, got %q", resolved)
		}
	})

	t.Run("explicit remote ref used as-is", func(t *testing.T) {
		upstream := initTestRepo(t)
		if _, err := git.RunGit(upstream, "branch", "feature-x"); err != nil {
			t.Fatalf("branch: %v", err)
		}
		clone := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, clone); err != nil {
			t.Fatalf("clone: %v", err)
		}

		resolved, found, err := resolveBaseRef("repo", clone, "origin/feature-x")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if resolved != "origin/feature-x" {
			t.Fatalf("expected origin/feature-x, got %q", resolved)
		}
	})

	t.Run("unqualified ref falls back to default remote", func(t *testing.T) {
		upstream := initTestRepo(t)
		clone := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, clone); err != nil {
			t.Fatalf("clone: %v", err)
		}
		// Create the branch on upstream AFTER cloning so the clone has no
		// remote-tracking ref for it yet — the fallback must fetch it.
		if _, err := git.RunGit(upstream, "branch", "pr5-branch"); err != nil {
			t.Fatalf("branch: %v", err)
		}

		resolved, found, err := resolveBaseRef("repo", clone, "pr5-branch")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if !found {
			t.Fatal("expected found via origin fallback")
		}
		if resolved != "origin/pr5-branch" {
			t.Fatalf("expected origin/pr5-branch, got %q", resolved)
		}
	})

	t.Run("local ref preferred over remote fallback", func(t *testing.T) {
		upstream := initTestRepo(t)
		if _, err := git.RunGit(upstream, "branch", "shared"); err != nil {
			t.Fatalf("branch: %v", err)
		}
		clone := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, clone); err != nil {
			t.Fatalf("clone: %v", err)
		}
		// A local branch with the same name exists.
		if _, err := git.RunGit(clone, "branch", "shared", "origin/shared"); err != nil {
			t.Fatalf("branch: %v", err)
		}

		resolved, found, err := resolveBaseRef("repo", clone, "shared")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if resolved != "shared" {
			t.Fatalf("expected local ref shared, got %q", resolved)
		}
	})

	t.Run("not found anywhere", func(t *testing.T) {
		upstream := initTestRepo(t)
		clone := filepath.Join(t.TempDir(), "clone")
		if _, err := git.RunGit(t.TempDir(), "clone", upstream, clone); err != nil {
			t.Fatalf("clone: %v", err)
		}

		resolved, found, err := resolveBaseRef("repo", clone, "does-not-exist")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if found {
			t.Fatal("expected not found")
		}
		if resolved != "does-not-exist" {
			t.Fatalf("expected original ref returned, got %q", resolved)
		}
	})

	t.Run("not found with no remotes", func(t *testing.T) {
		repo := initTestRepo(t)
		resolved, found, err := resolveBaseRef("repo", repo, "missing")
		if err != nil {
			t.Fatalf("resolveBaseRef: %v", err)
		}
		if found {
			t.Fatal("expected not found")
		}
		if resolved != "missing" {
			t.Fatalf("expected original ref returned, got %q", resolved)
		}
	})
}
