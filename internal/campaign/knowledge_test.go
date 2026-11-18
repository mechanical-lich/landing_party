package campaign

import (
	"encoding/json"
	"math"
	"testing"
)

// A new expedition knows only its landfall system: every other system and
// the gate are seen points, unidentified.
func TestNewExpeditionKnowsOnlyItsLandfall(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 5; seed++ {
		c := genCampaign(t, seed)
		known, total := c.IdentifiedCount()
		if known != 1 || total != len(c.Systems())+1 {
			t.Errorf("seed %d: %d of %d identified, want only the landfall system", seed, known, total)
		}
		if !c.StartLocation().Identified || c.GateLocation().Identified || !c.Locations[AdriftLocationID].Identified {
			t.Errorf("seed %d: start %v gate %v", seed, c.StartLocation().Identified, c.GateLocation().Identified)
		}
		if len(c.MapLocations()) == len(c.Locations) {
			t.Errorf("seed %d: the Star Map draws every location on day one", seed)
		}
	}
}

// A new expedition sees exactly what lies within sight of its landfall, plus
// the system its opening contract names; nothing else is on the map.
func TestNewExpeditionSeesOnlyItsSightRange(t *testing.T) {
	useRepoData(t)
	for _, size := range []string{"small", "medium", "large"} {
		var counts []int
		for seed := int64(1); seed <= 6; seed++ {
			c, err := GenerateCampaignOpts("Sight", seed, GenOptions{Size: size})
			if err != nil {
				t.Fatal(err)
			}
			wreck := c.Locations[AdriftLocationID]
			named := map[string]bool{}
			for _, q := range c.AvailableQuests() {
				named[q.Location] = true
			}
			r := c.SightRange()
			for _, l := range c.Locations {
				if !l.GalaxyPoint() {
					if !c.IsSeen(l) {
						t.Errorf("%s seed %d: %s (not a system) unseen", size, seed, l.ID)
					}
					continue
				}
				in := math.Hypot(l.X-wreck.X, l.Y-wreck.Y) <= r
				if c.IsSeen(l) != (in || named[l.ID]) {
					t.Errorf("%s seed %d: %s at %.0f seen %v (named %v)", size, seed, l.ID, math.Hypot(l.X-wreck.X, l.Y-wreck.Y), c.IsSeen(l), named[l.ID])
				}
			}
			counts = append(counts, c.SeenCount())
			for _, l := range c.MapLocations() {
				if !c.IsSeen(l) {
					t.Errorf("%s seed %d: the map lists unseen %s", size, seed, l.ID)
				}
			}
		}
		t.Logf("%s: seen at the start %v", size, counts)
	}
}

// Seen is saved per location and survives a round trip.
func TestSeenSurvivesSave(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 2)
	var far *Location
	for _, l := range c.Systems() {
		if !c.IsSeen(l) {
			far = l
			break
		}
	}
	if far == nil {
		t.Fatal("everything seen")
	}
	c.See(far)
	before := c.SeenCount()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	back := &Campaign{}
	if err := json.Unmarshal(data, back); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCampaign(back); err != nil {
		t.Fatal(err)
	}
	if back.SeenCount() != before || !back.IsSeen(back.Locations[far.ID]) || back.Locations[far.ID].Identified {
		t.Fatalf("after load: seen %d want %d, %s seen %v", back.SeenCount(), before, far.ID, back.IsSeen(back.Locations[far.ID]))
	}
}

// The backdrop is a function of the galaxy seed, shape and radius only:
// the same however far the expedition has got, and different seeds differ.
func TestBackdropDependsOnSeedAndShapeOnly(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 4)
	a := c.Backdrop()
	if len(a) < 100 {
		t.Fatalf("backdrop of %d specks", len(a))
	}
	for _, l := range c.Systems() {
		c.Identify(l)
	}
	c.CurrentLocationID = c.Systems()[3].ID
	b := c.Backdrop()
	if len(a) != len(b) {
		t.Fatal("backdrop changed with knowledge")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("backdrop changed with knowledge")
		}
	}
	d := GalaxyBackdrop(c.Seed, c.Galaxy.Shape, c.Galaxy.Radius)
	if len(d) != len(a) || d[0] != a[0] {
		t.Fatal("GalaxyBackdrop differs from the campaign's")
	}
	if e := GalaxyBackdrop(c.Seed+1, c.Galaxy.Shape, c.Galaxy.Radius); len(e) > 0 && e[0] == a[0] {
		t.Fatal("another seed gave the same backdrop")
	}
	for _, st := range a {
		if math.Hypot(st.X, st.Y) > c.Galaxy.Radius+1e-9 {
			t.Fatalf("speck outside the galaxy's radius: %+v", st)
		}
	}
}

// A quest never points at a system the player cannot see: every offered or
// active quest's system is on the map, after travel quests and charts too.
func TestQuestsNeverPointAtUnseen(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 4; seed++ {
		c := genCampaign(t, seed)
		for i := 0; i < 30; i++ {
			c.MaybeTravelQuests()
		}
		c.RevealCharts(3)
		for _, q := range append(c.AvailableQuests(), c.ActiveQuests()...) {
			if l := c.Locations[q.Location]; l != nil && !c.IsSeen(l) {
				t.Errorf("seed %d: quest %s points at unseen %s", seed, q.ID, l.ID)
			}
		}
	}
}

// A scan identifies every system within its tier's range, the gate as the
// gate, and nothing beyond; a second scan from the same place finds nothing
// new. Better scanners reach farther.
func TestScanIdentifiesEverythingInRange(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 3)
	gate := c.GateLocation()
	c.CurrentLocationID = c.Galaxy.ControlsID // beside the gate
	cur := c.CurrentLocation()
	found, ran := c.Scan(1)
	if !ran {
		t.Fatal("scan did not run")
	}
	r := ScanRange(1)
	for _, l := range c.Locations {
		if l.Kind != GateKind && (!l.HasSite() || l.Tutorial) {
			continue
		}
		in := math.Hypot(l.X-cur.X, l.Y-cur.Y) <= r
		if in != l.Identified && l.ID != c.StartID && l.ID != cur.ID {
			t.Errorf("%s at %.0f: identified %v", l.ID, math.Hypot(l.X-cur.X, l.Y-cur.Y), l.Identified)
		}
	}
	if !gate.Identified {
		t.Error("a scan beside the gate did not identify it")
	}
	if len(found) == 0 {
		t.Fatal("found nothing")
	}
	for i := 1; i < len(found); i++ {
		if math.Hypot(found[i].X-cur.X, found[i].Y-cur.Y) < math.Hypot(found[i-1].X-cur.X, found[i-1].Y-cur.Y) {
			t.Error("found is not nearest first")
		}
	}
	if again, _ := c.Scan(1); len(again) != 0 {
		t.Errorf("a second scan found %d new", len(again))
	}
	if ScanRange(3) <= ScanRange(2) || ScanRange(2) <= ScanRange(1) || ScanRange(0) != 0 {
		t.Errorf("ranges %v %v %v", ScanRange(1), ScanRange(2), ScanRange(3))
	}
}

// A system's own posted quests are offered only once it is identified; news
// (contracts, a travel roll's quests) is offered at once, and accepting it
// identifies its system.
func TestQuestsOfferedByKnowledge(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 4)
	for _, q := range c.AvailableQuests() {
		if l := c.Locations[q.Location]; l != nil && !l.Identified && !q.Offered {
			t.Fatalf("%s offered on unidentified %s", q.ID, l.ID)
		}
	}
	var news *Quest
	for i := 0; i < 200 && news == nil; i++ {
		c.MaybeTravelQuests()
		for _, q := range c.AvailableQuests() {
			if q.Offered && !c.Locations[q.Location].Identified {
				news = q
			}
		}
	}
	if news == nil {
		t.Fatal("no news on an unidentified system in 200 travel rolls")
	}
	if !c.AcceptQuest(news.ID) || !c.Locations[news.Location].Identified {
		t.Errorf("accepting %s did not identify %s", news.ID, news.Location)
	}
	// Star charts identify the nearest unknown systems.
	before, _ := c.IdentifiedCount()
	got := c.RevealCharts(3)
	after, _ := c.IdentifiedCount()
	if len(got) != 3 || after != before+3 {
		t.Errorf("charts: %d revealed, %d -> %d", len(got), before, after)
	}
}
