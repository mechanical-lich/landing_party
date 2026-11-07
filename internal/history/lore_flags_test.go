package history

import "testing"

// Lore flags name the chapter's event and civ and what the history left.
func TestLoreFlagsDescribeTheSite(t *testing.T) {
	lib := loreLibrary(t)
	found := 0
	for seed := int64(1); seed <= 200 && found < 20; seed++ {
		h := New(seed)
		site, err := h.BindSite(lib, "moon", seed*131)
		if err != nil {
			t.Fatal(err)
		}
		f := h.LoreFlags(lib, site)
		if f["population"] != site.State.Population {
			t.Errorf("population flag %v, site %s", f["population"], site.State.Population)
		}
		if (f["has_reactor"] == 1.0) != Has(site.State.Remains, "reactor") || (f["has_wreck"] == 1.0) != Has(site.State.Remains, "wrecks") {
			t.Errorf("remains flags %v for %v", f, site.State.Remains)
		}
		if site.Chapter == "" {
			if f["origin_event"] != "" {
				t.Errorf("quiet site has origin_event %v", f["origin_event"])
			}
			continue
		}
		found++
		ch := h.Event(site.Chapter)
		if f["origin_event"] != ch.Type {
			t.Errorf("origin_event %v, chapter %s", f["origin_event"], ch.Type)
		}
		if len(ch.Civs) > 0 && f["origin_civ"] != h.Civs[ch.Civs[0]].Name {
			t.Errorf("origin_civ %v, chapter civ %s", f["origin_civ"], h.Civs[ch.Civs[0]].Name)
		}
	}
	if found == 0 {
		t.Fatal("no moon site had a chapter")
	}
}
