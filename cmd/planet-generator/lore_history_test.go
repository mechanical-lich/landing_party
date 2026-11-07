package main

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/config"
)

// With "Scenario from history" on, generating plays the history's scenario and
// the info panel lists the chronicle.
func TestLoreScenarioFromHistory(t *testing.T) {
	cfg := config.Global()
	cfg.ScreenWidth, cfg.ScreenHeight = screenW, screenH
	cfg.WorldWidth, cfg.WorldHeight = screenW, screenH
	found := false
	for _, seed := range []string{"7", "4242", "99", "123456"} {
		o := testOptions(t, "moon")
		o.seedText = seed
		o.rollSize = false
		o.w, o.h, o.z = 80, 80, 10
		o.fromHistory = true
		a := &app{opts: o}
		a.generate()
		if a.lab == nil {
			t.Fatalf("seed %s: generation failed: %s", seed, a.genErr)
		}
		site := o.lore.site
		if site == nil {
			t.Fatalf("seed %s: no history: %s", seed, o.lore.err)
		}
		if o.sc == nil || o.sc.ID != site.Scenario {
			t.Errorf("seed %s: generated %v, history plays %s", seed, o.sc, site.Scenario)
		}
		a.click(0, 0) // builds every panel, the history section included
		if len(site.Events) > 0 && len(o.lore.origins(nil)) > 0 && !o.lore.resolved(a.lab.Level()) {
			t.Errorf("seed %s: level not built from the history (no resolved origins)", seed)
		}
		if len(o.lore.origins(a.lab.Level())) > 0 {
			found = true
		}
		a.lab.Close()
	}
	if !found {
		t.Error("no seed gave an event with an origin")
	}
}

// The tutorial map has no history; the panel says so instead of failing.
func TestLoreTutorialHasNoHistory(t *testing.T) {
	o := testOptions(t, "tutorial")
	o.rollHistory()
	if o.lore.site != nil || o.lore.err == "" {
		t.Errorf("tutorial got a history: %+v", o.lore)
	}
}

// Picking a scenario by hand turns history picking off.
func TestLoreHandPickedScenarioWins(t *testing.T) {
	a := testApp(t)
	a.opts.fromHistory = true
	// Click down the right-hand arrows of the options panel: the first cycle
	// is Map, the second Scenario, which must turn history picking off.
	for y := 0; y < 200 && a.opts.fromHistory; y += 4 {
		a.click(leftW-20, y)
	}
	if a.opts.fromHistory {
		t.Error("changing the scenario left history picking on")
	}
}
