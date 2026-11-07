package main

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/generation"
)

// The history overlay (lore plan, step 8d): the site's tier, a budget readout
// within its limits, every origin labelled with its event and what stands
// there, and the landing reserve with its keep-outs.
func TestContentHistoryOverlay(t *testing.T) {
	cfg := config.Global()
	cfg.ScreenWidth, cfg.ScreenHeight = screenW, screenH
	cfg.WorldWidth, cfg.WorldHeight = screenW, screenH
	stamped := false
	for _, seed := range []string{"7", "4242", "99", "123456"} {
		o := testOptions(t, "earth_like")
		o.seedText = seed
		o.rollSize = false
		o.w, o.h, o.z = 80, 80, 10
		o.fromHistory = true
		a := &app{opts: o}
		a.generate()
		if a.lab == nil {
			t.Fatalf("seed %s: generation failed: %s", seed, a.genErr)
		}
		l := o.lore
		if l.site == nil {
			t.Fatalf("seed %s: no history: %s", seed, l.err)
		}
		if l.tier == "" {
			t.Errorf("seed %s: no tier", seed)
		}
		b := l.ev.Budget
		if b.Stamps > b.Limits.Stamps && l.site.Chapter == "" {
			t.Errorf("seed %s: %d stamps over the limit %d", seed, b.Stamps, b.Limits.Stamps)
		}
		if lines := l.budgetLines(a.lab.Level()); len(lines) < 3 || !strings.Contains(lines[1], "stamps") {
			t.Errorf("seed %s: budget readout %q", seed, lines)
		}
		for _, id := range l.originIDs() {
			label := l.originLabel(id, false, nil)
			if !strings.HasPrefix(label, l.h.Event(id).Type) {
				t.Errorf("seed %s: origin %s labelled %q", seed, id, label)
			}
			for _, fb := range l.ev.Features {
				if fb.Kind == "stamp" && fb.At == "origin:"+id {
					stamped = true
					if s, _ := fb.Params["script"].(string); !strings.Contains(label, s) {
						t.Errorf("seed %s: origin %s label %q lacks its stamp %s", seed, id, label, s)
					}
				}
			}
		}
		if _, _, _, ok := generation.Landing(a.lab.Level()); !ok {
			t.Errorf("seed %s: no landing reserve on the level", seed)
		}
		if k := l.keepOuts(); len(k) < 2 || k[0] != generation.Budget().KeepOut {
			t.Errorf("seed %s: keep-outs %v", seed, k)
		}
		a.click(0, 0) // builds the history panel with the readout and labels
		a.lab.Close()
	}
	if !stamped {
		t.Error("no seed stamped a structure at an origin")
	}
}
