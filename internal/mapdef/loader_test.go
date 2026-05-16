package mapdef

import "testing"

func TestLoadDataMaps(t *testing.T) {
	if err := Load("../../data/maps"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(All()) == 0 {
		t.Fatal("expected at least one map definition")
	}
	for _, want := range []string{"earth_like", "alien", "moon", "asteroid", "abandoned_station"} {
		m := ByID(want)
		if m == nil {
			t.Errorf("missing map %q", want)
			continue
		}
		if m.Terrain == "" {
			t.Errorf("map %q has empty terrain", want)
		}
	}
	if Random() == nil {
		t.Fatal("Random returned nil with maps loaded")
	}
}
