package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/history"
)

func loreTicks(wm *WorldManager, live *MainState, n int) {
	for i := 0; i < n; i++ {
		wm.TickBackground(live)
	}
}

// The live level's frames are world ticks: a day of them is a star-date day.
func TestLoreStarDateAdvancesWithLiveTicks(t *testing.T) {
	wm, live := bgAway(t, "a")
	start := wm.Campaign.StarDate
	loreTicks(wm, live, history.TicksPerDay*2+10)
	if got := wm.Campaign.StarDate - start; got != 2 {
		t.Errorf("star date moved %d days, want 2", got)
	}
	if wm.Campaign.StarTick != 10 {
		t.Errorf("star tick = %d, want 10", wm.Campaign.StarTick)
	}
}

// On the dashboard nothing is live, but away planets tick and so does time.
func TestLoreStarDateAdvancesWithBackgroundOnly(t *testing.T) {
	wm, _ := bgAway(t, "a")
	start := wm.Campaign.StarDate
	loreTicks(wm, nil, history.TicksPerDay)
	if got := wm.Campaign.StarDate - start; got != 1 {
		t.Errorf("star date moved %d days on the dashboard, want 1", got)
	}
}

func TestLoreStarDateStopsWhilePaused(t *testing.T) {
	wm, live := bgAway(t, "a")
	wm.SetPaused(true)
	before, tick := wm.Campaign.StarDate, wm.Campaign.StarTick
	loreTicks(wm, live, history.TicksPerDay*3)
	loreTicks(wm, nil, history.TicksPerDay)
	if wm.Campaign.StarDate != before || wm.Campaign.StarTick != tick {
		t.Errorf("paused clock moved: %d+%d -> %d+%d", before, tick, wm.Campaign.StarDate, wm.Campaign.StarTick)
	}
}

// A jump costs ceil(fuel/3) days; levels left behind keep their own day.
func TestLoreStarDateAdvancesByJumpCost(t *testing.T) {
	wm, _ := bgAway(t, "a")
	wm.Campaign.Locations["a"].X = 10 // live sits at the origin: 10 fuel away
	before := wm.Campaign.StarDate
	bgTravel(t, wm, "a")
	if got := wm.Campaign.StarDate - before; got != 4 {
		t.Errorf("jump of 10 fuel took %d days, want 4", got)
	}
	before = wm.Campaign.StarDate
	bgTravel(t, wm, "a") // already there: no jump
	if wm.Campaign.StarDate != before {
		t.Error("travelling to the current location took time")
	}
}

func TestLoreStarDateSurvivesSaveLoad(t *testing.T) {
	wm, live := bgAway(t, "a")
	loreTicks(wm, live, history.TicksPerDay*5+123)
	if err := wm.SaveCampaign(live); err != nil {
		t.Fatalf("save: %v", err)
	}
	c, err := LoadCampaign(wm.Campaign.Name)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.StarDate != wm.Campaign.StarDate || c.StarTick != wm.Campaign.StarTick {
		t.Errorf("loaded %d+%d, saved %d+%d", c.StarDate, c.StarTick, wm.Campaign.StarDate, wm.Campaign.StarTick)
	}
}

// A planet frozen to disk sits still; on return its HUD shows the campaign's date,
// not the day its own clock stopped at.
func TestLoreFrozenPlanetShowsCampaignDate(t *testing.T) {
	wm := bgWorld(t, []string{"b"}, "a")
	bgTravel(t, wm, "a")
	bgTravel(t, wm, "b")
	if wm.IsLoaded("a") {
		t.Fatal("unstaffed planet should have frozen")
	}
	loreTicks(wm, wm.Current(), history.TicksPerDay*7)
	bgTravel(t, wm, "a")
	a := wm.Current()
	if a.level.Day == wm.Campaign.Day() {
		t.Fatalf("test needs the frozen level's day (%d) behind the campaign's", a.level.Day)
	}
	want := "Star Date: 7"
	if got := a.dateLine(); !strings.Contains(got, want) {
		t.Errorf("HUD date = %q, want %q", got, want)
	}
}

// Version 1 campaigns predate the history and are refused, not migrated.
func TestLoreVersionOneCampaignRefused(t *testing.T) {
	b0Campaign(t, "a")
	root := campaignRoot("Old")
	writeOldCampaign(t, root, &campaign.Campaign{
		Version:   1,
		Name:      "Old",
		Locations: map[string]*campaign.Location{"a": {ID: "a"}},
	})
	_, err := LoadCampaign("Old")
	if err == nil {
		t.Fatal("version 1 campaign loaded")
	}
	if !strings.Contains(err.Error(), "older version") {
		t.Errorf("unclear refusal: %v", err)
	}
}
