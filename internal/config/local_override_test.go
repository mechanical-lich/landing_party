package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfigDir lays out data/config.json and data/config.local.json in a
// temp dir and chdirs there, the way Global() finds them.
func writeConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "config.json"), []byte(`{"screenWidth": 640}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "config.local.json"), []byte(`{"observerMode": true, "screenWidth": 999}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// Under go test the player's config.local.json is ignored: a local
// observerMode: true must not reach Global().
func TestLocalOverridesIgnoredUnderTest(t *testing.T) {
	writeConfigDir(t)
	saved := settings
	t.Cleanup(func() { settings = saved })
	Reload()
	if !localOverridesDisabled() {
		t.Fatal("local overrides should be disabled under go test")
	}
	if Global().ObserverMode {
		t.Fatal("observerMode from config.local.json leaked into a test")
	}
	if Global().ScreenWidth != 640 {
		t.Fatalf("screenWidth = %d, want 640 from config.json", Global().ScreenWidth)
	}
}

// The merge the game runs still applies the file (only the test gate skips it).
func TestLocalOverridesMergeStillApplies(t *testing.T) {
	dir := writeConfigDir(t)
	cfg, err := LoadConfig(filepath.Join(dir, "data", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	applyLocalOverridesFrom(cfg, filepath.Join(dir, "data", "config.local.json"))
	if !cfg.ObserverMode || cfg.ScreenWidth != 999 {
		t.Fatalf("merge not applied: observer=%v width=%d", cfg.ObserverMode, cfg.ScreenWidth)
	}
}
