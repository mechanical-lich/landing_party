package campaign

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// genConfig keeps the loaded file between checks, and picks up an edit at
// the next check; a caller's changes to the slices do not reach the cache.
func TestGenConfigCacheReloadsEditedFile(t *testing.T) {
	old := GenerationConfigPath
	t.Cleanup(func() { GenerationConfigPath = old })
	GenerationConfigPath = filepath.Join(t.TempDir(), "generation.json")

	if got, want := genConfig().JumpFuelPerDay, defaultGenConfig().JumpFuelPerDay; got != want {
		t.Fatalf("no file: jump fuel per day %d, want the default %d", got, want)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(GenerationConfigPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	recheck := func() { // the next call looks at the file again
		genCache.Lock()
		genCache.checked = time.Time{}
		genCache.Unlock()
	}

	write(`{"jump_fuel_per_day": 7}`)
	if got := genConfig().JumpFuelPerDay; got != defaultGenConfig().JumpFuelPerDay {
		t.Fatalf("before the next check: jump fuel per day %d, want the kept default", got)
	}
	recheck()
	if got := genConfig().JumpFuelPerDay; got != 7 {
		t.Fatalf("after the check: jump fuel per day %d, want 7", got)
	}
	write(`{"jump_fuel_per_day": 11}`)
	recheck()
	if got := genConfig().JumpFuelPerDay; got != 11 {
		t.Fatalf("after an edit: jump fuel per day %d, want 11", got)
	}

	c := genConfig()
	c.ScanFuelCosts[0] = -1
	if genConfig().ScanFuelCosts[0] == -1 {
		t.Fatal("a caller's change to ScanFuelCosts reached the cache")
	}
}
