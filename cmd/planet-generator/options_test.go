package main

import (
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/world"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	if err := world.LoadTileDefinitionsDir("data/tiledefinitions"); err != nil {
		panic(err)
	}
	if err := factory.FactoryLoadDir(config.Global().BlueprintPath); err != nil {
		panic(err)
	}
	if err := game.LoadPlanetLabData(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func testOptions(t *testing.T, mapID string) *options {
	t.Helper()
	o, err := newOptions()
	if err != nil {
		t.Fatal(err)
	}
	if !o.selectMapByID(mapID) {
		t.Fatalf("no map %q", mapID)
	}
	return o
}

func TestSelectMapPicksNamesakeScenario(t *testing.T) {
	for mapID, want := range map[string]string{
		"earth_like": "earth_like", "alien": "alien_world", "asteroid": "asteroid_field",
	} {
		o := testOptions(t, mapID)
		if o.sc == nil || o.sc.ID != want {
			t.Errorf("map %s: scenario = %v, want %s", mapID, o.sc, want)
		}
	}
	if o := testOptions(t, "desert_planet"); o.sc != nil {
		t.Errorf("desert_planet: scenario = %s, want none", o.sc.ID)
	}
}

// Edits go to private copies: the loaded definitions must survive so picking the
// map again resets it, and the game's own lookups stay clean.
func TestEditsDoNotTouchLoadedDefinitions(t *testing.T) {
	o := testOptions(t, "earth_like")
	o.md.TerrainParams["mountain_frequency"] = 0.99
	o.features[0].enabled = false
	o.md.Features[0].Count = 999
	o.sc.HostileMax = 999

	orig := mapdef.ByID("earth_like")
	if orig.TerrainParams["mountain_frequency"] == 0.99 {
		t.Error("terrain param edit reached the loaded map")
	}
	if orig.Features[0].Count == 999 {
		t.Error("feature edit reached the loaded map")
	}
	for _, sc := range o.scenarios {
		if sc.ID == "earth_like" && sc.HostileMax == 999 {
			t.Error("scenario edit reached the picker's copy")
		}
	}
}

func TestSpecAppliesOptions(t *testing.T) {
	o := testOptions(t, "earth_like")
	total := len(o.md.Features)
	o.seedText = "77"
	o.rollSize = false
	o.w, o.h, o.z = 80, 90, 10
	o.features[0].enabled = false
	o.features[1].count, o.features[1].touched = 3, true
	o.biomeFeatures = false
	o.setupScripts = false
	o.spawns = false
	o.lightIdx, o.ambient = 2, 40

	s := o.spec()
	if s.Width != 80 || s.Height != 90 || s.Depth != 10 || s.Seed != 77 {
		t.Errorf("size/seed = %dx%dx%d seed %d", s.Width, s.Height, s.Depth, s.Seed)
	}
	if len(s.Map.Features) != total-1 {
		t.Errorf("features = %d, want %d", len(s.Map.Features), total-1)
	}
	if f := s.Map.Features[0]; f.Count != 3 || f.CountMax != 0 {
		t.Errorf("touched count = %d-%d, want fixed 3", f.Count, f.CountMax)
	}
	if !s.Map.SkipBiomeFeatures || !s.SuppressSpawns || len(s.Scenario.SetupScripts) != 0 {
		t.Errorf("toggles not applied: %+v", s)
	}
	if s.LightingMode != "fixed" || s.LightingAmbient != 40 {
		t.Errorf("lighting = %s %d", s.LightingMode, s.LightingAmbient)
	}
	if len(o.sc.SetupScripts) == 0 {
		t.Error("spec dropped scripts from the editor's scenario, so re-enabling can't restore them")
	}
	if len(o.md.Features) != total {
		t.Error("spec dropped features from the editor's map")
	}
}

func TestRollSizeMatchesTheGame(t *testing.T) {
	o := testOptions(t, "moon")
	o.seedText = "1234"
	s := o.spec()
	w, h, z := mapdef.ByID("moon").Size.Roll(1234)
	if s.Width != w || s.Height != h || s.Depth != z {
		t.Errorf("size %dx%dx%d, game rolls %dx%dx%d", s.Width, s.Height, s.Depth, w, h, z)
	}
}

func TestSeedFallsBackToNonzero(t *testing.T) {
	o := testOptions(t, "moon")
	for _, txt := range []string{"", "0", "-"} {
		o.seedText = txt
		if o.seed() == 0 {
			t.Errorf("seed %q resolved to 0, which means random", txt)
		}
	}
}

// uiAt returns a ui inside one panel with the mouse at (mx,my).
func uiAt(mx, my int) *ui {
	return &ui{mx: mx, my: my, panel: rect{0, 0, 200, 400}, y: 10}
}

func TestToggleFlipsOnlyWhenClicked(t *testing.T) {
	v := false
	u := uiAt(20, 15)
	if u.toggle("x", &v); v {
		t.Error("hover alone flipped the toggle")
	}
	u = uiAt(20, 15)
	u.pressed = true
	if !u.toggle("x", &v) || !v {
		t.Error("click did not flip the toggle")
	}
	u = uiAt(20, 300)
	u.pressed = true
	if u.toggle("x", &v); !v {
		t.Error("click elsewhere flipped the toggle")
	}
}

func TestCycleWrapsBothWays(t *testing.T) {
	items := []string{"a", "b", "c"}
	i := 0
	u := uiAt(panelPad+5, 10+15+5) // left arrow
	u.pressed = true
	if !u.cycle("n", items, &i) || i != 2 {
		t.Errorf("left from 0 = %d, want 2", i)
	}
	u = uiAt(200-panelPad-5, 10+15+5) // right arrow
	u.pressed = true
	if !u.cycle("n", items, &i) || i != 0 {
		t.Errorf("right from 2 = %d, want 0", i)
	}
}

func TestSliderDragClampsAndSnaps(t *testing.T) {
	v := 0.0
	u := uiAt(panelPad+90, 28) // midpoint of a 180px track
	u.pressed, u.down = true, true
	if !u.slider("s", "n", &v, 0, 10, 1) || v != 5 {
		t.Errorf("drag to midpoint = %v, want 5", v)
	}
	// The drag keeps tracking the mouse outside the row, clamped to the range.
	u.y, u.pressed, u.mx, u.my = 10, false, 5000, 300
	if !u.slider("s", "n", &v, 0, 10, 1) || v != 10 {
		t.Errorf("drag past the end = %v, want 10", v)
	}
	// Without a drag started on it, a held mouse changes nothing.
	u = uiAt(panelPad+90, 28)
	u.down = true
	v = 1
	if u.slider("s", "n", &v, 0, 10, 1); v != 1 {
		t.Errorf("hover with button held changed value to %v", v)
	}
}

func TestPanelScrollClampsToContent(t *testing.T) {
	u := &ui{wheelY: -100, mx: 10, my: 10}
	r := rect{0, 0, 100, 100}
	u.beginPanel("p", r)
	u.y += 150
	u.endPanel()
	if got := u.scroll["p"]; got <= 0 || got > 150 {
		t.Errorf("scroll = %d, want clamped to the overflow", got)
	}
	u.wheelY = 1000
	u.beginPanel("p", r)
	if u.scroll["p"] != 0 {
		t.Errorf("scrolled past the top: %d", u.scroll["p"])
	}
}
