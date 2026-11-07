package game

import "testing"

// The crashed ship is at the history's origin, with its crew for a battle
// crash and empty for a scout that misjudged its landing.
func TestLoreCrashedUFOAtTheOrigin(t *testing.T) {
	for ch, crew := range map[string]bool{"ship_shot_down": true, "scout_accident": false} {
		flags := map[string]any{"origin_civ": "Vael"}
		if !crew {
			flags["chapter_crew"] = "dead"
		}
		ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "crashed_ufo", Seed: 47, Chapter: ch, Flags: flags})
		if hull, _ := loreNear(ms.level, []string{"storage_locker"}, ox, oy, 12); hull == 0 {
			t.Errorf("%s: no hull at the origin", ch)
		}
		near, _ := loreNear(ms.level, loreUFOCrew, ox, oy, 12)
		if crew && near < 5 || !crew && near != 0 {
			t.Errorf("%s: %d crew at the hull", ch, near)
		}
	}
}

// Without a history the hull lands where the stamp's own search puts it, crew
// and all, exactly as before.
func TestLoreCrashedUFOUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "crashed_ufo", "crashed_ufo_setup", 47, append([]string{"storage_locker"}, loreUFOCrew...))
}
