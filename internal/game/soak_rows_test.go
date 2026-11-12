package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// soakProgress reports how far one soak run is: the returned func, called
// with the tick just stepped, prints a line at each tenth of the run (not
// more often than every five seconds, so a short run prints only its last
// line) with the time taken and an estimate of what is left. The lines go
// to stderr, or are appended to the file LP_SOAK_PROGRESS names
// (scripts/soak_par.sh follows that file, the seeds' own output being in
// their logs). Wall time only reaches the output, never the simulation.
func soakProgress(label string, ticks int) func(tick int) {
	start := time.Now()
	last := start
	next := 1 // the next tenth to report
	return func(tick int) {
		if ticks <= 0 || tick*10 < next*ticks {
			return
		}
		next = tick*10/ticks + 1
		now := time.Now()
		if tick < ticks && now.Sub(last) < 5*time.Second {
			return
		}
		last = now
		el := now.Sub(start)
		line := fmt.Sprintf("soak %-14s %3d%%  %d/%d ticks  %s", label, tick*100/ticks, tick, ticks, el.Round(time.Second))
		if tick < ticks {
			left := time.Duration(float64(el) * float64(ticks-tick) / float64(tick))
			line += fmt.Sprintf(", about %s left", left.Round(time.Second))
		} else {
			line += ", done"
		}
		if path := os.Getenv("LP_SOAK_PROGRESS"); path != "" {
			if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				fmt.Fprintln(f, line)
				f.Close()
				return
			}
		}
		fmt.Fprintln(os.Stderr, line)
	}
}

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
