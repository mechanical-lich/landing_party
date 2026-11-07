package game

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// TestContentPresetsBuild builds every structure viewer preset
// (data/history/structure_presets.json), intact and ruined, at several seeds:
// each stays in its footprint, leaves no posts or stacked furniture and
// builds in time (lore plan, step 8d). The seeds must not all build the same
// layout: a preset comes out in more than one orientation or arrangement.
func TestContentPresetsBuild(t *testing.T) {
	loreLoadData(t)
	presets, err := history.LoadPresets(filepath.Join("data", "history"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range presets {
		layouts := map[string]bool{}
		for seed := int64(1); seed <= 6; seed++ {
			for _, damage := range []float64{0, 3} {
				params := map[string]any{"damage": damage}
				for k, v := range p.Params {
					params[k] = v
				}
				n++
				level, z := contentBuild(t, p.Script, p.W, p.H, params, seed, "*")
				if damage == 0 {
					layouts[contentLayoutKey(contentTileMap(level, 4, 4, p.W, p.H, z))] = true
				}
			}
		}
		if len(layouts) < 2 {
			t.Errorf("preset %q: every seed built the same layout", p.Name)
		}
	}
	if n < 100 {
		t.Fatalf("built only %d presets", n)
	}
}

// contentLayoutKey flattens a tile map into a comparable string.
func contentLayoutKey(m map[[2]int]string) string {
	keys := make([]string, 0, len(m))
	for p, v := range m {
		keys = append(keys, fmt.Sprintf("%d,%d=%s", p[0], p[1], v))
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}
