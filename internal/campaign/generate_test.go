package campaign

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

// useRepoData points the data-driven loaders at the real templates/config in
// the repo's data/ directory (tests run from the package dir).
func useRepoData(t *testing.T) {
	t.Helper()
	LocationTemplatePath = "../../data/location_templates.json"
	QuestTemplatePath = "../../data/quest_templates.json"
	GenerationConfigPath = "../../data/generation.json"
	genCfgCache = nil // force reload against the repo config
}

func genCampaign(t *testing.T, seed int64) *Campaign {
	t.Helper()
	c, err := GenerateCampaign("Test", seed)
	if err != nil {
		t.Fatalf("GenerateCampaign: %v", err)
	}
	return c
}

func TestGenerateCampaignShape(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 42)

	if c.CurrentLocationID == "" {
		t.Fatal("no start location set")
	}
	if start := c.Locations[c.CurrentLocationID]; start == nil || start.Kind == HomeKind {
		t.Fatalf("start location invalid: %+v", start)
	}
	home := c.HomeLocation()
	if home == nil {
		t.Fatal("no Home location")
	}
	if !home.Discovered {
		t.Fatal("Home should be visible from the start")
	}
	if len(c.QuestDefs) == 0 {
		t.Fatal("no quests generated")
	}
	// Start + Home + at least two nearby systems, all discovered.
	discovered := 0
	for _, l := range c.Locations {
		if l.Discovered {
			discovered++
		}
	}
	if discovered < 4 {
		t.Fatalf("expected >=4 discovered locations (start+home+nearby), got %d", discovered)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	useRepoData(t)
	a := genCampaign(t, 1234)
	b := genCampaign(t, 1234)

	if a.CurrentLocationID != b.CurrentLocationID {
		t.Fatalf("start differs: %s vs %s", a.CurrentLocationID, b.CurrentLocationID)
	}
	if len(a.Locations) != len(b.Locations) || len(a.QuestDefs) != len(b.QuestDefs) {
		t.Fatalf("size differs: locs %d/%d quests %d/%d",
			len(a.Locations), len(b.Locations), len(a.QuestDefs), len(b.QuestDefs))
	}
	for id, la := range a.Locations {
		lb := b.Locations[id]
		if lb == nil || la.X != lb.X || la.Y != lb.Y || la.ScenarioID != lb.ScenarioID {
			t.Fatalf("location %s differs across identical seeds", id)
		}
	}
	// Different seed should (almost certainly) produce a different layout.
	if d := genCampaign(t, 9999); d.CurrentLocationID == a.CurrentLocationID &&
		len(d.Locations) == len(a.Locations) {
		t.Log("warning: seed 9999 produced same-size layout as 1234 (not necessarily a bug)")
	}
}

func TestFuelCostIsDistance(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 7)

	if got := c.FuelCost(c.CurrentLocationID); got != 0 {
		t.Fatalf("fuel to current location should be 0, got %d", got)
	}
	for id, loc := range c.Locations {
		if id == c.CurrentLocationID {
			continue
		}
		fc := c.FuelCost(id)
		if loc.Kind == HomeKind {
			if fc < 100 {
				t.Fatalf("Home should be expensive, got %d", fc)
			}
			continue
		}
		if fc <= 0 {
			t.Fatalf("nearby system %s has non-positive fuel cost %d (x=%.1f y=%.1f)", id, fc, loc.X, loc.Y)
		}
	}
}

func TestLocationBoundQuestRevealAndScope(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 3)

	// Find a generated contract: location-bound, available, its location
	// hidden until accepted (datapad quests are excluded — they start hidden).
	var q *Quest
	for i := range c.QuestDefs {
		d := &c.QuestDefs[i]
		if d.Location == "" || d.Delivery == "datapad" {
			continue
		}
		if c.Quests[d.ID].Status != QuestAvailable {
			continue
		}
		if loc := c.Locations[d.Location]; loc != nil && !loc.Discovered {
			q = d
			break
		}
	}
	if q == nil {
		t.Skip("no hidden contract quest in this seed")
	}
	loc := c.Locations[q.Location]

	if c.Quests[q.ID].Status != QuestAvailable {
		t.Fatalf("contract should start available, got %s", c.Quests[q.ID].Status)
	}
	if !c.AcceptQuest(q.ID) {
		t.Fatal("AcceptQuest failed")
	}
	if !loc.Discovered {
		t.Fatal("accepting a location-bound quest must reveal its location")
	}

	// Off-site: objective met elsewhere must NOT complete it.
	c.CurrentLocationID = "elsewhere"
	c.EvaluateQuests(objective.EvalContext{
		EntityCounts: map[string]int{},
		ResourceCounts: map[string]int{
			q.Objective.Resource: q.Objective.Threshold + 100,
		},
	})
	if c.Quests[q.ID].Status == QuestCompleted {
		t.Fatal("location-bound quest completed while off-site")
	}
}

func TestBountyQuestRegistersFixture(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 3)
	// Roll a lot of travel quests so a bounty contract is very likely.
	for i := 0; i < 200; i++ {
		c.MaybeTravelQuests()
	}
	found := false
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if q.Objective.Trigger != objective.TriggerTargetKilled {
			continue
		}
		found = true
		if q.Objective.Target != q.ID {
			t.Fatalf("bounty %s objective target = %q, want its own id", q.ID, q.Objective.Target)
		}
		loc := c.Locations[q.Location]
		if loc == nil {
			t.Fatalf("bounty %s bound to missing location %q", q.ID, q.Location)
		}
		var fx *QuestFixture
		for j := range loc.Fixtures {
			if loc.Fixtures[j].QuestID == q.ID {
				fx = &loc.Fixtures[j]
			}
		}
		if fx == nil {
			t.Fatalf("bounty %s has no fixture on its location", q.ID)
		}
		if fx.Blueprint == "" || fx.Spawned {
			t.Fatalf("fixture for %s invalid: %+v", q.ID, *fx)
		}
	}
	if !found {
		t.Skip("no bounty quest generated for this seed/run")
	}

	// AddFixture must dedupe by quest id.
	loc := &Location{}
	loc.AddFixture(QuestFixture{QuestID: "z", Blueprint: "a"})
	loc.AddFixture(QuestFixture{QuestID: "z", Blueprint: "b"})
	if len(loc.Fixtures) != 1 || loc.Fixtures[0].Blueprint != "a" {
		t.Fatalf("AddFixture should dedupe by quest id, got %+v", loc.Fixtures)
	}
}

func TestDatapadQuestsGenerated(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 11)

	dpQuests := 0
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if q.Delivery != "datapad" {
			continue
		}
		dpQuests++
		// Hidden until its datapad is recovered.
		if c.Quests[q.ID].Status != QuestHidden {
			t.Fatalf("datapad quest %s should be hidden, got %s", q.ID, c.Quests[q.ID].Status)
		}
		loc := c.Locations[q.Location]
		if loc == nil {
			t.Fatalf("datapad quest %s bound to missing location %q", q.ID, q.Location)
		}
		found := false
		for _, fx := range loc.Fixtures {
			if fx.QuestID == q.ID {
				if fx.Kind != "datapad" || fx.Blueprint != "datapad" {
					t.Fatalf("fixture for %s wrong: %+v", q.ID, fx)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("datapad quest %s has no datapad fixture on its location", q.ID)
		}
	}
	if dpQuests == 0 {
		t.Fatal("no datapad quests generated")
	}
	t.Logf("generated %d datapad quests across %d locations", dpQuests, len(c.Locations))
}

func TestTravelRollRateAndProgression(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 55)
	baseQuests := len(c.QuestDefs)

	hits := 0
	for i := 0; i < 240; i++ {
		if _, added := c.MaybeTravelQuests(); added > 0 {
			hits++
			if added < 1 || added > 4 {
				t.Fatalf("added out of 1d4 range: %d", added)
			}
		}
	}
	// ~1/6 of 240 ≈ 40; allow a wide band so it never flakes.
	if hits < 10 || hits > 80 {
		t.Fatalf("travel-roll hit rate implausible: %d/240 (expected ~40)", hits)
	}
	if len(c.QuestDefs) <= baseQuests {
		t.Fatal("travel rolls never added quests")
	}
}

func TestTravelRollDeterministic(t *testing.T) {
	useRepoData(t)
	a := genCampaign(t, 88)
	b := genCampaign(t, 88)
	for i := 0; i < 50; i++ {
		_, na := a.MaybeTravelQuests()
		_, nb := b.MaybeTravelQuests()
		if na != nb {
			t.Fatalf("jump %d: non-deterministic travel roll (%d vs %d)", i, na, nb)
		}
	}
	if !strings.HasPrefix(a.CurrentLocationID, "start") {
		t.Logf("start id = %s", a.CurrentLocationID)
	}
}
