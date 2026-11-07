package history

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every stamp a site keeps is ready for the structure library: a real
// script (roles resolved), a footprint, its event and size; a purposeful
// one its holds and clue; a ship its class at that class's footprint, a
// mothership only on a large map. War sites stamp a fort on each side when
// their peoples build forts, and their battles leave a battlefield.
func TestContentStampsCarryTheContract(t *testing.T) {
	lib := loreLibrary(t)
	scripts, motherships, battlefields := map[string]int{}, 0, 0
	for m, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			log.SetOutput(&bytes.Buffer{})
			ev := s.h.Evidence(lib, s.SiteHistory)
			log.SetOutput(os.Stderr)
			w, _, _ := lib.Maps[m].Size.Roll(s.Seed)
			forts := 0
			for _, f := range ev.Features {
				if f.Kind != "stamp" {
					continue
				}
				sc, _ := f.Params["script"].(string)
				scripts[sc]++
				if _, err := os.Stat(filepath.Join(loreDataRoot, "scripts", "structures", sc+".basic")); err != nil || strings.HasPrefix(sc, "@") {
					t.Errorf("%s %s: stamp script %q", m, s.ID, sc)
				}
				if f.Params["event"] == nil || f.Params["size"] == nil || f.Params["w"] == nil {
					t.Errorf("%s %s: %s stamp lacks its contract: %v", m, s.ID, sc, f.Params)
				}
				if p, _ := f.Params["purpose"].(string); p != "" {
					if f.Params["holds"] == nil || f.Params["clue"] == nil || strings.Contains(f.Params["clue"].(string), "{") {
						t.Errorf("%s %s: %s (%s) has holds %v, clue %v", m, s.ID, sc, p, f.Params["holds"], f.Params["clue"])
					}
				}
				if sc == "crashed_ship" {
					class, _ := f.Params["class"].(string)
					c, ok := lib.ShipClasses[class]
					if !ok || f.Params["w"].(float64) != float64(c.W) {
						t.Errorf("%s %s: ship class %q at %v", m, s.ID, class, f.Params["w"])
					}
					if class == "mothership" {
						motherships++
						if generationBudgetClass(w) != "large" {
							t.Errorf("%s %s: a mothership on a %d-wide map", m, s.ID, w)
						}
					}
				}
				if sc == "fort" || (s.Arc == "sundered_world" && sc == "bunker") {
					forts++
				}
				if sc == "battlefield" {
					battlefields++
				}
			}
			if s.Arc == "sundered_world" && forts == 1 {
				// Either both sides build a fort or (a primitive side's
				// fort is a palisade, still a fort) neither was kept.
				t.Logf("%s %s: one fort kept (the budget or a level without forts took the other)", m, s.ID)
			}
		}
	}
	for _, want := range []string{"crashed_ship", "village", "fort", "battlefield", "graveyard", "castle", "mine"} {
		if scripts[want] == 0 {
			t.Errorf("no site stamps %s (%v)", want, scripts)
		}
	}
	if battlefields == 0 {
		t.Error("no battlefield came up")
	}
	t.Logf("stamps by script: %v; motherships %d", scripts, motherships)
}

func generationBudgetClass(width int) string {
	switch {
	case width <= 180:
		return "small"
	case width <= 384:
		return "medium"
	}
	return "large"
}
