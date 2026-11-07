package history

import "testing"

// loreRepeats counts (civ, beat event type) pairs that occur more than once
// across a campaign's sites: the same civ opening two sealed labs.
func loreRepeats(lib *Library, h *History) int {
	seen := map[string]int{}
	for _, e := range h.Events {
		def := lib.Events[e.Type]
		if def == nil || e.Site == "" || !beat(def) || len(e.Civs) == 0 {
			continue
		}
		seen[beatKey(e.Civs[0], e.Type)]++
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n += c - 1
		}
	}
	return n
}

// A civ's memorable beats (a figure, a place on the map) rarely repeat across a
// campaign: only when nothing else satisfies a stage.
func TestLoreBeatsRarelyRepeatForACiv(t *testing.T) {
	lib := loreLibrary(t)
	const seeds, sites = 200, 12
	total, worst, beats := 0, 0, 0
	for seed := int64(1); seed <= seeds; seed++ {
		h := New(seed)
		loreBindMany(t, lib, h, 0, sites)
		r := loreRepeats(lib, h)
		total += r
		// A pool of one or two civs can't help repeating them.
		if len(h.Pool) >= 3 && r > worst {
			worst = r
		}
		for _, e := range h.Events {
			if def := lib.Events[e.Type]; def != nil && e.Site != "" && beat(def) {
				beats++
			}
		}
	}
	t.Logf("%d repeated beats in %d (%.2f per %d-site campaign), worst campaign of 3+ civs %d", total, beats, float64(total)/seeds, sites, worst)
	if per := float64(total) / seeds; per > 0.3 {
		t.Errorf("%.2f repeated beats per campaign, want at most 0.3", per)
	}
	if worst > 2 {
		t.Errorf("a campaign of 3+ civs repeated %d beats, want at most 2", worst)
	}
}
