package history

import (
	"strings"
	"testing"
)

// half reports which side of the map an origin lies in along either axis:
// -1 below 45%, +1 above 55%, 0 in the border band (per axis).
func contentHalves(p *SitePoint, w, h int) (int, int) {
	f := func(v, span int) int {
		switch {
		case v < span*45/100:
			return -1
		case v >= span*55/100:
			return 1
		}
		return 0
	}
	return f(p.X, w), f(p.Y, h)
}

// Every sundered_world site has two peoples with a fort each in opposite
// halves of the map, battles in the band between, each side's datapad told in
// its own voice, and an ending its fates agree with.
func TestContentWarArcBothSides(t *testing.T) {
	lib := loreLibrary(t)
	wars, endings := 0, map[string]int{}
	for m, sites := range contentSites(t, lib, 500, false) {
		for _, s := range sites {
			if s.Arc != "sundered_world" {
				continue
			}
			wars++
			w, hh, _ := lib.Maps[m].Size.Roll(s.Seed)
			var forts []*Event
			civs := map[string]bool{}
			for _, id := range s.Events {
				ev := s.h.Event(id)
				switch ev.Type {
				case "fortify":
					forts = append(forts, ev)
				case "great_battle", "great_battle_bloody", "border_drawn":
					if x, y := contentHalves(ev.Origin, w, hh); x != 0 && y != 0 {
						t.Errorf("%s %s: %s at %v, outside the border band", m, s.ID, ev.Type, *ev.Origin)
					}
				case "mutual_ruin", "mutual_annihilation", "one_side_prevails":
					endings[ev.Type]++
				}
				if def := lib.Events[ev.Type]; def.PerSide && def.TextSide != "" {
					side := s.h.RenderSide(lib, ev, "X")
					if side == s.h.Render(lib, ev, "X") || !strings.Contains(side, s.h.Civs[ev.Civs[0]].Name) {
						t.Errorf("%s %s: %s's datapad is not its people's account: %q", m, s.ID, ev.Type, side)
					}
				}
				if def := lib.Events[ev.Type]; def.PerSide && ev.Vars["meanwhile"] != "" && s.h.Render(lib, ev, "X") == s.h.Render(lib, s.h.Event(ev.Causes[0]), "X") {
					t.Errorf("%s %s: %s tells the same line for both sides", m, s.ID, ev.Type)
				}
				if ev.Type == "twin_peoples_rise" {
					civs[ev.Civs[0]] = true
				}
			}
			if len(civs) != 2 {
				t.Errorf("%s %s: %d peoples rose, want 2", m, s.ID, len(civs))
				continue
			}
			if len(forts) != 2 || forts[0].Civs[0] == forts[1].Civs[0] {
				t.Errorf("%s %s: forts %v, want one per side", m, s.ID, forts)
				continue
			}
			ax, ay := contentHalves(forts[0].Origin, w, hh)
			bx, by := contentHalves(forts[1].Origin, w, hh)
			if !(ax != 0 && bx != 0 && ax != bx) && !(ay != 0 && by != 0 && ay != by) {
				t.Errorf("%s %s: forts at %v and %v are not on opposite sides", m, s.ID, *forts[0].Origin, *forts[1].Origin)
			}
			died := 0
			for c := range civs {
				if s.Outcomes[c] == OutcomeDied {
					died++
				}
			}
			if died == 0 || !Has([]string{StatusFallen, StatusDead}, s.State.Status) {
				t.Errorf("%s %s: war ended with %d peoples dead, status %s", m, s.ID, died, s.State.Status)
			}
		}
	}
	if wars < 20 {
		t.Errorf("%d war sites in 500 seeds", wars)
	}
	for _, e := range []string{"mutual_ruin", "mutual_annihilation", "one_side_prevails"} {
		if endings[e] == 0 {
			t.Errorf("no war ended in %s (%v)", e, endings)
		}
	}
}

// The variants come up: a visitor uplifts a primitive people by two levels,
// an overlord enslaves one, a survivor people becomes a remnant hazard.
func TestContentWarVariants(t *testing.T) {
	lib := loreLibrary(t)
	seen := map[string]int{}
	for _, sites := range contentSites(t, lib, 500, false) {
		for _, s := range sites {
			seen[s.Arc]++
			for _, id := range s.Events {
				ev := s.h.Event(id)
				seen[ev.Type]++
				if ev.Type == "uplift_contact" && s.h.Civs[ev.Civs[1]].Tech == TechPrimitive {
					t.Errorf("%s: an uplifted people stayed primitive", s.ID)
				}
			}
		}
	}
	for _, k := range []string{"uplifted_people", "enslaved_world", "uplift_gift", "schism", "uprising_won", "uprising_crushed", "remnant_band"} {
		if seen[k] == 0 {
			t.Errorf("%s never came up in 500 seeds", k)
		}
	}
}
