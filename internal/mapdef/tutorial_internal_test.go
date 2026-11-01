package mapdef

import "testing"

// Random never rolls an internal map, however many times it is asked.
func TestRandomSkipsInternalMaps(t *testing.T) {
	prev := loaded
	t.Cleanup(func() { loaded = prev })
	loaded = []MapDef{
		{ID: "normal_a"},
		{ID: "tutorial", Internal: true},
		{ID: "normal_b"},
	}
	for i := 0; i < 500; i++ {
		m := Random()
		if m == nil {
			t.Fatal("Random returned nil with non-internal maps loaded")
		}
		if m.Internal {
			t.Fatalf("Random returned internal map %q", m.ID)
		}
	}
	loaded = []MapDef{{ID: "tutorial", Internal: true}}
	if m := Random(); m != nil {
		t.Fatalf("Random returned %q when only internal maps are loaded", m.ID)
	}
	if ByID("tutorial") == nil {
		t.Fatal("ByID must still find internal maps")
	}
}
