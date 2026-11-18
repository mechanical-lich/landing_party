package campaign

import (
	"encoding/json"
	"testing"
)

func lockCampaign() *Campaign {
	c := &Campaign{Name: "Lock", CurrentLocationID: "start", Locations: map[string]*Location{
		"start":    {ID: "start", Identified: true},
		"tutorial": {ID: "tutorial", Identified: true, Tutorial: true},
		"gate":     {ID: "gate", Kind: GateKind, Identified: true},
		"sys_3":    {ID: "sys_3", Identified: true},
		"sys_9":    {ID: "sys_9", Identified: false}, // a hidden contract destination
	}}
	c.Tutorial = &TutorialProgress{NextLocationID: "start", Unlocked: []string{"tutorial"}}
	return c
}

// At the start only the Training Ground is open; Home, neighbours and hidden
// contracts are locked with the first-landing reason, and the current location
// never reports a lock.
func TestTutorialTravelLockAtStart(t *testing.T) {
	c := lockCampaign()
	if got := c.TutorialTravelLock("tutorial"); got != "" {
		t.Errorf("tutorial planet locked: %q", got)
	}
	for _, id := range []string{"home", "sys_3", "sys_9"} {
		if got := c.TutorialTravelLock(id); got != lockFirstLanding {
			t.Errorf("%s lock = %q, want %q", id, got, lockFirstLanding)
		}
	}
	if got := c.TutorialTravelLock("start"); got != "" {
		t.Errorf("current location locked: %q", got)
	}
}

// Unlocking the second destination opens it and changes the reason elsewhere;
// Done, Skipped and a nil Tutorial lift the lock everywhere.
func TestTutorialTravelLockLifts(t *testing.T) {
	c := lockCampaign()
	c.CurrentLocationID = "tutorial"
	if got := c.TutorialTravelLock("start"); got != lockFirstLanding {
		t.Fatalf("start before unlock = %q", got)
	}
	c.Tutorial.Unlock(c.Tutorial.NextLocationID)
	c.Tutorial.Unlock(c.Tutorial.NextLocationID) // once only
	if len(c.Tutorial.Unlocked) != 2 {
		t.Fatalf("unlocked = %v", c.Tutorial.Unlocked)
	}
	if got := c.TutorialTravelLock("start"); got != "" {
		t.Errorf("next destination still locked: %q", got)
	}
	if got := c.TutorialTravelLock("sys_3"); got != lockFirstBuild+"start" {
		t.Errorf("sys_3 lock = %q, want %q", got, lockFirstBuild)
	}
	if got := c.TutorialTravelLock("tutorial"); got != "" {
		t.Errorf("current location locked: %q", got)
	}

	done := lockCampaign()
	done.Tutorial.Done = true
	skipped := lockCampaign()
	skipped.Tutorial.Skipped = true
	none := lockCampaign()
	none.Tutorial = nil
	for name, cc := range map[string]*Campaign{"done": done, "skipped": skipped, "nil": none} {
		for id := range cc.Locations {
			if got := cc.TutorialTravelLock(id); got != "" {
				t.Errorf("%s: %s locked: %q", name, id, got)
			}
		}
		if cc.TutorialActive() {
			t.Errorf("%s: tutorial reports active", name)
		}
	}
	var nilC *Campaign
	if got := nilC.TutorialTravelLock("x"); got != "" {
		t.Errorf("nil campaign locked: %q", got)
	}
}

// A save made on the second planet must not relock the way back: the lock
// answers the same after a JSON round trip.
func TestTutorialTravelLockSurvivesReload(t *testing.T) {
	c := lockCampaign()
	c.Tutorial.Unlock("start")
	c.CurrentLocationID = "start"
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Campaign
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"start", "tutorial", "home", "sys_3", "sys_9"} {
		if a, b := c.TutorialTravelLock(id), back.TutorialTravelLock(id); a != b {
			t.Errorf("%s: lock %q before save, %q after", id, a, b)
		}
	}
	if back.TutorialTravelLock("tutorial") != "" || back.TutorialTravelLock("sys_3") != lockFirstBuild+"start" {
		t.Errorf("reloaded lock state wrong: %v", back.Tutorial)
	}
}
