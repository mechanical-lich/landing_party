package campaign

import "testing"

// A presence's unseen-event list is bounded: an observer run (which shows
// every faction event anyway) keeps none, a normal campaign the most recent
// maxPresenceEvents (secondary finding: it grew for every event).
func TestPresenceEventsBounded(t *testing.T) {
	c, _, f := shipTestCampaign(t)
	p := &f.Presence
	p.Known = false
	d := mirrorDoctrine(t)
	loc := c.Locations["home"]
	for i := 0; i < 3*maxPresenceEvents; i++ {
		c.recordFaction("faction_grew", loc, p, d, map[string]string{"stage": "camp"})
	}
	if len(p.Events) != maxPresenceEvents {
		t.Errorf("%d unseen events kept, want %d", len(p.Events), maxPresenceEvents)
	}
	last := p.Events[len(p.Events)-1]
	if c.History.Events[len(c.History.Events)-1].ID != last {
		t.Error("the cap dropped the newest event")
	}
	c.Observer = true
	p.Events = nil
	for i := 0; i < 10; i++ {
		c.recordFaction("faction_grew", loc, p, d, map[string]string{"stage": "camp"})
	}
	if len(p.Events) != 0 {
		t.Errorf("an observer run kept %d unseen events", len(p.Events))
	}
}
