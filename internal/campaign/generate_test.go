package campaign

import (
	"math"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// useRepoData points the data-driven loaders at the real templates/config in
// the repo's data/ directory (tests run from the package dir).
func useRepoData(t *testing.T) {
	t.Helper()
	LocationTemplatePath = "../../data/location_templates.json"
	QuestTemplatePath = "../../data/quest_templates.json"
	GenerationConfigPath = "../../data/generation.json"
	lore.NamesPath = "../../data/names.json"
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

func TestBlendAngle(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	sameDir := func(x, y float64) bool { return near(math.Cos(x), math.Cos(y)) && near(math.Sin(x), math.Sin(y)) }

	if got := blendAngle(1.0, 2.0, 0); !near(got, 1.0) {
		t.Errorf("t=0 should be a: %v", got)
	}
	if got := blendAngle(1.0, 2.0, 0.5); !near(got, 1.5) {
		t.Errorf("t=0.5 midpoint: %v", got)
	}
	if got := blendAngle(1.0, 2.0, 1); !sameDir(got, 2.0) {
		t.Errorf("t=1 should be b: %v", got)
	}
	// Shortest arc across the ±pi seam (should go the short way, not +2pi).
	if got := blendAngle(3.0, -3.0, 1); !sameDir(got, -3.0) {
		t.Errorf("seam: %v", got)
	}
	if got := blendAngle(0.2, -0.2, 1); !near(got, -0.2) {
		t.Errorf("small arc: %v", got)
	}
}

// Expansion should trend toward Home (most new systems in the home direction,
// and the frontier moving closer) while still branching laterally so the player
// has other directions to explore.
func TestExpansionBranchesTowardHome(t *testing.T) {
	useRepoData(t)
	arc := func(d float64) float64 {
		d = math.Mod(d, 2*math.Pi)
		if d < -math.Pi {
			d += 2 * math.Pi
		} else if d > math.Pi {
			d -= 2 * math.Pi
		}
		return d
	}
	minToHome := func(c *Campaign, hx, hy float64) float64 {
		m := math.MaxFloat64
		for _, l := range c.Locations {
			if l.Kind == HomeKind {
				continue
			}
			if d := math.Hypot(hx-l.X, hy-l.Y); d < m {
				m = d
			}
		}
		return m
	}
	for _, seed := range []int64{1, 4242} {
		c := genCampaign(t, seed)
		home := c.HomeLocation()
		homeAng := math.Atan2(home.Y, home.X)
		before := minToHome(c, home.X, home.Y)
		existing := map[string]bool{}
		for id := range c.Locations {
			existing[id] = true
		}
		c.Expand(40)

		fwd, lat := 0, 0
		for id, l := range c.Locations {
			if existing[id] || l.Kind == HomeKind {
				continue
			}
			off := math.Abs(arc(math.Atan2(l.Y, l.X) - homeAng))
			if off < math.Pi/4 {
				fwd++
			} else if off < 3*math.Pi/4 {
				lat++
			}
		}
		if fwd < 5 {
			t.Errorf("seed %d: only %d systems trend toward home", seed, fwd)
		}
		if lat < 3 {
			t.Errorf("seed %d: only %d lateral systems — not branching", seed, lat)
		}
		if after := minToHome(c, home.X, home.Y); after > before {
			t.Errorf("seed %d: expansion moved the frontier away from home (%.0f -> %.0f)", seed, before, after)
		}
	}
}

func TestScannerResearchLevels(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 1)
	if c.ScannerLevel() != 0 {
		t.Fatalf("fresh scanner level = %d, want 0", c.ScannerLevel())
	}
	if c.ScanFuelCost() != 200 {
		t.Fatalf("base scan cost = %d, want 200", c.ScanFuelCost())
	}
	c.UnlockTech("ship_scanner_1")
	c.UnlockTech("ship_scanner_2")
	c.UnlockTech("ship_scanner_3")
	if c.ScannerLevel() != 3 {
		t.Fatalf("scanner level = %d, want 3", c.ScannerLevel())
	}
	c.UnlockTech("scan_efficiency_1")
	if c.ScanFuelCost() != 100 {
		t.Fatalf("scan cost after efficiency_1 = %d, want 100", c.ScanFuelCost())
	}
	c.UnlockTech("scan_efficiency_2")
	if c.ScanFuelCost() != 50 {
		t.Fatalf("scan cost after efficiency_2 = %d, want 50", c.ScanFuelCost())
	}
}

func TestScanReveals(t *testing.T) {
	useRepoData(t)
	// Level 0 (no scanner) never scans (and doesn't "run").
	if c0 := genCampaign(t, 7); func() bool { l, r := c0.Scan(0); return l != nil || r }() {
		t.Fatal("level 0 should not scan")
	}

	c := genCampaign(t, 7)
	c.UnlockTech("ship_scanner_1")
	c.UnlockTech("ship_scanner_2")
	c.UnlockTech("ship_scanner_3") // level 3: 30% / range 100
	cur := c.Locations[c.CurrentLocationID]

	var revealed *Location
	for i := 0; i < 100 && revealed == nil; i++ {
		revealed, _ = c.Scan(3)
	}
	if revealed == nil {
		t.Fatal("no reveal in 100 scans at 30%")
	}
	if !revealed.Discovered {
		t.Error("revealed location should be discovered")
	}
	if d := math.Hypot(revealed.X-cur.X, revealed.Y-cur.Y); d > 100+1e-6 {
		t.Errorf("revealed %.1f from current, exceeds range 100", d)
	}
}

func TestScanDeterministic(t *testing.T) {
	useRepoData(t)
	run := func() (int, float64, float64) {
		c := genCampaign(t, 55)
		c.UnlockTech("ship_scanner_1")
		n := 0
		var lx, ly float64
		for i := 0; i < 20; i++ {
			if loc, _ := c.Scan(1); loc != nil {
				n++
				lx, ly = loc.X, loc.Y
			}
		}
		return n, lx, ly
	}
	n1, x1, y1 := run()
	n2, x2, y2 := run()
	if n1 != n2 || x1 != x2 || y1 != y2 {
		t.Fatalf("scan non-deterministic: (%d,%.2f,%.2f) vs (%d,%.2f,%.2f)", n1, x1, y1, n2, x2, y2)
	}
}

func TestTooClose(t *testing.T) {
	c := &Campaign{Locations: map[string]*Location{
		"a": {X: 0, Y: 0},
		"b": {X: 100, Y: 0},
	}}
	if !c.tooClose(3, 4, 6) { // distance 5 from "a", under minSep 6
		t.Fatal("a point 5 away should be too close at minSep 6")
	}
	if c.tooClose(50, 0, 6) { // far from both
		t.Fatal("the midpoint should not be too close")
	}
	if c.tooClose(0.1, 0, 0) { // minSep <= 0 disables the check
		t.Fatal("minSep <= 0 must disable the separation check")
	}
}

// Every generated location should sit at least MinSeparation from every other
// (so star-map icons don't overlap) and carry a unique name.
func TestGenerateSeparationAndUniqueNames(t *testing.T) {
	useRepoData(t)
	minSep := genConfig().MinSeparation
	if minSep <= 0 {
		t.Fatalf("expected positive min_separation from repo config, got %v", minSep)
	}
	for _, seed := range []int64{1, 7, 42, 99, 1234, 20240} {
		c := genCampaign(t, seed)
		names := make(map[string]string) // name -> owning id
		locs := make([]*Location, 0, len(c.Locations))
		for id, l := range c.Locations {
			if prev, dup := names[l.Name]; dup {
				t.Fatalf("seed %d: duplicate name %q on %s and %s", seed, l.Name, prev, id)
			}
			names[l.Name] = id
			locs = append(locs, l)
		}
		for i := 0; i < len(locs); i++ {
			for j := i + 1; j < len(locs); j++ {
				d := math.Hypot(locs[i].X-locs[j].X, locs[i].Y-locs[j].Y)
				if d < minSep {
					t.Fatalf("seed %d: %s and %s are %.2f apart (< min_separation %.1f)",
						seed, locs[i].ID, locs[j].ID, d, minSep)
				}
			}
		}
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
		// Valid if it either names a creature (legacy: spawned in the
		// open) or names a structure (the generator script spawns and
		// flags the target). Must not be pre-Spawned.
		if (fx.Blueprint == "" && fx.Structure == "") || fx.Spawned {
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
