package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// TestContentPresetsMatchData checks structure_presets.json against the data
// it is drawn from: every ship purpose at its class's footprint, every tech
// level's roles at both footprints, every preset's script on disk.
func TestContentPresetsMatchData(t *testing.T) {
	lib, err := LoadLibrary(filepath.Join(loreDataRoot, "history"), mapdef.All())
	if err != nil {
		t.Fatal(err)
	}
	presets, err := LoadPresets(filepath.Join(loreDataRoot, "history"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range presets {
		have[fmt.Sprintf("%s %dx%d %v %v", p.Script, p.W, p.H, p.Params["purpose"], p.Params["tech"])] = true
		path := filepath.Join(loreDataRoot, "scripts", "structures", p.Script+".basic")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("preset %q: script %s missing", p.Name, p.Script)
		}
	}
	for id, pd := range lib.Purposes {
		if pd.Class == "" {
			continue
		}
		sc := lib.ShipClasses[pd.Class]
		if k := fmt.Sprintf("crashed_ship %dx%d %s <nil>", sc.W, sc.H, id); !have[k] {
			t.Errorf("no preset %s", k)
		}
	}
	for _, td := range lib.TechDefs {
		for role, alts := range td.Structures {
			purpose := any(roleDefaultPurpose[role])
			if purpose == "" {
				purpose = nil
			}
			for _, st := range alts {
				for _, size := range []string{"large", "small"} {
					w, h := st.Footprint(size)
					if k := fmt.Sprintf("%s %dx%d %v %s", st.Script, w, h, purpose, td.ID); !have[k] {
						t.Errorf("no preset %s (%s %s)", k, td.ID, role)
					}
				}
			}
		}
	}
}
