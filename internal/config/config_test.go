package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevdoran/projector/internal/config"
)

// withTempHome patches HOME (and unsets XDG_CONFIG_HOME) to an isolated temp
// dir so config resolution stays self-contained. Returns the temp home.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

// newConfigPath returns the path to the new-location config file under home:
// <home>/.config/projector/config.toml.
func newConfigPath(home string) string {
	return filepath.Join(home, ".config", "projector", "config.toml")
}

// legacyConfigPath returns the path to the legacy config file under home:
// <home>/.projector/projector-config.toml.
func legacyConfigPath(home string) string {
	return filepath.Join(home, ".projector", "projector-config.toml")
}

// writeFile writes content to path, creating parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
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
		RepoSearchDirs: []string{"/tmp/repos1", "/tmp/repos2"},
		DefaultEditor:  "cursor",
		Editors: map[string]config.EditorConfig{
			"myeditor": {Name: "My Editor", Command: "myedit", Terminal: true},
		},
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
	if len(loaded.RepoSearchDirs) != 2 {
		t.Errorf("RepoSearchDirs len: got %d, want 2", len(loaded.RepoSearchDirs))
	}
	if loaded.DefaultEditor != "cursor" {
		t.Errorf("DefaultEditor: got %q, want %q", loaded.DefaultEditor, "cursor")
	}
	if loaded.Editors["myeditor"].Name != "My Editor" {
		t.Errorf("Editors[myeditor].Name: got %q", loaded.Editors["myeditor"].Name)
	}
	if loaded.Editors["myeditor"].Command != "myedit" {
		t.Errorf("Editors[myeditor].Command: got %q", loaded.Editors["myeditor"].Command)
	}
	if !loaded.Editors["myeditor"].Terminal {
		t.Error("Editors[myeditor].Terminal: got false, want true")
	}
	if loaded.Repos["my-repo"].DefaultBase != "origin/develop" {
		t.Errorf("Repos[my-repo].DefaultBase: got %q", loaded.Repos["my-repo"].DefaultBase)
	}
	if loaded.ConfigVersion != config.CurrentConfigVersion {
		t.Errorf("ConfigVersion: got %d, want %d", loaded.ConfigVersion, config.CurrentConfigVersion)
	}
}

func TestLoad_IgnoresOldEditorField(t *testing.T) {
	home := withTempHome(t)
	// Simulate an old config file with the deprecated "editor" field.
	writeFile(t, newConfigPath(home), `config-version = 1
projects-dir = "/tmp/projects"
editor = "code"
`)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The old editor field should be silently ignored; DefaultEditor should be empty.
	if loaded.DefaultEditor != "" {
		t.Errorf("DefaultEditor should be empty for old config, got %q", loaded.DefaultEditor)
	}
	if loaded.ProjectsDir != "/tmp/projects" {
		t.Errorf("ProjectsDir: got %q", loaded.ProjectsDir)
	}
}

func TestLoad_ErrConfigVersionTooNew(t *testing.T) {
	home := withTempHome(t)
	writeFile(t, newConfigPath(home), "config-version = 9999\nprojects-dir = \"/tmp/projects\"\n")
	_, err := config.Load()
	if !errors.Is(err, config.ErrConfigVersionTooNew) {
		t.Fatalf("expected ErrConfigVersionTooNew, got %v", err)
	}
}

func TestLoad_ParseError(t *testing.T) {
	home := withTempHome(t)
	writeFile(t, newConfigPath(home), "not = valid [toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// TestSave_WritesToNewLocation verifies Save targets the new XDG location.
func TestSave_WritesToNewLocation(t *testing.T) {
	home := withTempHome(t)
	if err := config.Save(&config.GlobalConfig{ProjectsDir: "/tmp/projects"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(newConfigPath(home)); err != nil {
		t.Errorf("expected config at new location %s: %v", newConfigPath(home), err)
	}
	if _, err := os.Stat(legacyConfigPath(home)); !os.IsNotExist(err) {
		t.Errorf("Save should not write to the legacy location")
	}
}

// TestConfigFilePath_DefaultsToXDGConfigHome verifies the path uses ~/.config
// when XDG_CONFIG_HOME is unset.
func TestConfigFilePath_DefaultsToXDGConfigHome(t *testing.T) {
	home := withTempHome(t)
	path, err := config.ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath: %v", err)
	}
	if path != newConfigPath(home) {
		t.Errorf("ConfigFilePath: got %q, want %q", path, newConfigPath(home))
	}
}

// TestConfigFilePath_HonorsXDGConfigHome verifies $XDG_CONFIG_HOME override.
func TestConfigFilePath_HonorsXDGConfigHome(t *testing.T) {
	withTempHome(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path, err := config.ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath: %v", err)
	}
	want := filepath.Join(xdg, "projector", "config.toml")
	if path != want {
		t.Errorf("ConfigFilePath: got %q, want %q", path, want)
	}
}

// TestLoad_PrefersNewOverLegacy verifies the new location wins when both exist.
func TestLoad_PrefersNewOverLegacy(t *testing.T) {
	home := withTempHome(t)
	writeFile(t, newConfigPath(home), "config-version = 1\nprojects-dir = \"/new/projects\"\n")
	writeFile(t, legacyConfigPath(home), "config-version = 1\nprojects-dir = \"/legacy/projects\"\n")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ProjectsDir != "/new/projects" {
		t.Errorf("ProjectsDir: got %q, want %q (new location should win)", loaded.ProjectsDir, "/new/projects")
	}
	// Legacy file must be left untouched.
	data, err := os.ReadFile(legacyConfigPath(home))
	if err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if !strings.Contains(string(data), "/legacy/projects") {
		t.Errorf("legacy file should be unchanged, got: %s", data)
	}
}

// TestLoad_MigratesLegacyToNew verifies that when only the legacy config
// exists, Load copies it to the new location and leaves the legacy file in place.
func TestLoad_MigratesLegacyToNew(t *testing.T) {
	home := withTempHome(t)
	legacyContent := "config-version = 1\nprojects-dir = \"/legacy/projects\"\n"
	writeFile(t, legacyConfigPath(home), legacyContent)

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ProjectsDir != "/legacy/projects" {
		t.Errorf("ProjectsDir: got %q, want %q", loaded.ProjectsDir, "/legacy/projects")
	}

	// New location must now exist with the migrated content.
	newData, err := os.ReadFile(newConfigPath(home))
	if err != nil {
		t.Fatalf("expected migrated config at new location: %v", err)
	}
	if string(newData) != legacyContent {
		t.Errorf("migrated content mismatch: got %q, want %q", newData, legacyContent)
	}

	// Legacy file must be left in place (copy, not move).
	if _, err := os.Stat(legacyConfigPath(home)); err != nil {
		t.Errorf("legacy file should be left in place after migration: %v", err)
	}

	// A subsequent Load should read from the new location without error.
	if _, err := config.Load(); err != nil {
		t.Fatalf("second Load after migration: %v", err)
	}
}

// TestLoad_NoConfigAnywhere verifies ErrNotFound when neither location exists.
func TestLoad_NoConfigAnywhere(t *testing.T) {
	withTempHome(t)
	if _, err := config.Load(); !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
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
