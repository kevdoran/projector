package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevdoran/projector/internal/config"
)

// writeCfgFile writes a TOML config file to a temp dir and patches HOME.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestLoad_ErrNotFound(t *testing.T) {
	withTempHome(t)
	_, err := config.Load()
	if !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSave_And_Load_RoundTrip(t *testing.T) {
	withTempHome(t)

	cfg := &config.GlobalConfig{
		ProjectsDir:    "/tmp/projects",
		TemplateDir:    "/tmp/templates",
		RepoSearchDirs: []string{"/tmp/repos1", "/tmp/repos2"},
		Repos: map[string]config.RepoConfig{
			"my-repo": {DefaultBase: "origin/develop"},
		},
	}

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ProjectsDir != cfg.ProjectsDir {
		t.Errorf("ProjectsDir: got %q, want %q", loaded.ProjectsDir, cfg.ProjectsDir)
	}
	if loaded.TemplateDir != cfg.TemplateDir {
		t.Errorf("TemplateDir: got %q, want %q", loaded.TemplateDir, cfg.TemplateDir)
	}
	if len(loaded.RepoSearchDirs) != 2 {
		t.Errorf("RepoSearchDirs len: got %d, want 2", len(loaded.RepoSearchDirs))
	}
	if loaded.Repos["my-repo"].DefaultBase != "origin/develop" {
		t.Errorf("Repos[my-repo].DefaultBase: got %q", loaded.Repos["my-repo"].DefaultBase)
	}
	if loaded.ConfigVersion != config.CurrentConfigVersion {
		t.Errorf("ConfigVersion: got %d, want %d", loaded.ConfigVersion, config.CurrentConfigVersion)
	}
}

func TestLoad_ErrConfigVersionTooNew(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".projector")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "config-version = 9999\nprojects-dir = \"/tmp/projects\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "projector-config.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load()
	if !errors.Is(err, config.ErrConfigVersionTooNew) {
		t.Fatalf("expected ErrConfigVersionTooNew, got %v", err)
	}
}

func TestLoad_ParseError(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".projector")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "projector-config.toml"), []byte("not = valid [toml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestResolveBase_PerRepoOverride(t *testing.T) {
	cfg := &config.GlobalConfig{
		Repos: map[string]config.RepoConfig{
			"my-repo": {DefaultBase: "origin/develop"},
		},
	}
	// With override, should return override without checking remote refs
	base, err := config.ResolveBase(cfg, "my-repo", "/nonexistent")
	if err != nil {
		t.Fatalf("ResolveBase: %v", err)
	}
	if base != "origin/develop" {
		t.Errorf("expected 'origin/develop', got %q", base)
	}
}

func TestResolveBase_FallbackToHEAD(t *testing.T) {
	cfg := &config.GlobalConfig{}
	// No remote refs in a non-repo dir → should fall back to HEAD
	base, err := config.ResolveBase(cfg, "unknown-repo", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBase: %v", err)
	}
	if base != "HEAD" {
		t.Errorf("expected 'HEAD', got %q", base)
	}
}

func TestValidate(t *testing.T) {
	if err := config.Validate(&config.GlobalConfig{ProjectsDir: "/tmp"}); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
	if err := config.Validate(&config.GlobalConfig{}); err == nil {
		t.Error("expected error for missing projects-dir")
	}
}
