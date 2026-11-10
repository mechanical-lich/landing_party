package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A broken factions file is an error at start-up, not a quiet empty set
// (secondary review note: a malformed factions.json was one stderr line and
// the player got a ship with no crew, build menu or research).
func TestValidateFactionData(t *testing.T) {
	prev := FactionsPath
	t.Cleanup(func() { FactionsPath = prev; factionDataCache = nil })
	dir := t.TempDir()
	for name, body := range map[string]string{
		"malformed.json": `{"doctrines": {"colony": `,
		"nocolony.json":  `{"doctrines": {"hive": {"name": "Hive"}}}`,
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		FactionsPath, factionDataCache = p, nil
		if err := ValidateFactionData(nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	FactionsPath, factionDataCache = filepath.Join(dir, "missing.json"), nil
	if err := ValidateFactionData(nil); err == nil {
		t.Error("a missing factions file: no error")
	}
	// The real file passes, and an unknown stage structure is caught.
	useRepoData(t)
	factionDataCache = nil
	if err := ValidateFactionData(func(string) bool { return true }); err != nil {
		t.Errorf("data/factions.json: %v", err)
	}
	err := ValidateFactionData(func(k string) bool { return k != "egg_cluster" })
	if err == nil || !strings.Contains(err.Error(), "egg_cluster") {
		t.Errorf("an unknown stage structure: %v", err)
	}
}
