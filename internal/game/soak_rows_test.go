package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The parallel soak harness (make soak-autonz, make soak-hive): the soaks
// share package globals (the settlement map, the path budget, the content
// tables), so their seeds cannot run as parallel subtests in one process.
// Instead the Makefile runs one test process per seed (LP_SOAK_ONLY names
// it) side by side, each writing its row as JSON into LP_SOAK_ROWS, and a
// last process joins the rows (the soak's Join test), prints the same table
// and applies the same thresholds as a one-process run.

// soakRowOut writes row into LP_SOAK_ROWS as kind-key.json when this
// process runs one seed of a parallel soak, and reports whether it did.
func soakRowOut(t *testing.T, kind, key string, row any) bool {
	t.Helper()
	dir := os.Getenv("LP_SOAK_ROWS")
	if dir == "" || os.Getenv("LP_SOAK_ONLY") == "" {
		return false
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, kind+"-"+key+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return true
}

// soakRowsIn reads every kind-*.json row in LP_SOAK_ROWS into rows (a
// pointer to a slice); false when there is no such directory set.
func soakRowsIn(t *testing.T, kind string, rows any) bool {
	t.Helper()
	dir := os.Getenv("LP_SOAK_ROWS")
	if dir == "" {
		return false
	}
	files, err := filepath.Glob(filepath.Join(dir, kind+"-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no %s rows in %s", kind, dir)
	}
	var raw []json.RawMessage
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, b)
	}
	joined, _ := json.Marshal(raw)
	if err := json.Unmarshal(joined, rows); err != nil {
		t.Fatal(err)
	}
	return true
}
