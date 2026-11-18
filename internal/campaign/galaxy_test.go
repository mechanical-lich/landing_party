package campaign

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

func galaxyBytes(t *testing.T, c *Campaign) []byte {
	t.Helper()
	b, err := json.Marshal(struct {
		G *Galaxy
		L map[string]*Location
		H *history.History
		Q []Quest
		S int
	}{c.Galaxy, c.Locations, c.History, c.QuestDefs, c.GenSeq})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The same seed and size give the same galaxy, byte for byte.
func TestGalaxySameSeedSameBytes(t *testing.T) {
	useRepoData(t)
	for _, size := range []string{"small", "medium", "large"} {
		for _, seed := range []int64{1, 42} {
			a, err := GenerateGalaxy(seed, size)
			if err != nil {
				t.Fatal(err)
			}
			b, err := GenerateGalaxy(seed, size)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(galaxyBytes(t, a), galaxyBytes(t, b)) {
				t.Errorf("%s seed %d: two generations differ", size, seed)
			}
			if want := genConfig().Galaxy.size(size).Systems; len(a.Systems()) != want {
				t.Errorf("%s seed %d: %d systems, want %d", size, seed, len(a.Systems()), want)
			}
		}
	}
}

// The galaxy draws nothing that depends on who arrives: the same seed with
// a different race, rival count or rival boxes makes the same systems.
func TestGalaxyIndependentOfExpedition(t *testing.T) {
	useRepoData(t)
	const seed = 7
	bare, err := GenerateGalaxy(seed, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []GenOptions{{}, {Factions: 3}, {Doctrine: "hive", Factions: 2}, {Rivals: []RivalPick{{Doctrine: "greyish"}}}} {
		c, err := GenerateCampaignOpts("X", seed, opts)
		if err != nil {
			t.Fatal(err)
		}
		for id, l := range bare.Locations {
			got := c.Locations[id]
			if got == nil || got.Name != l.Name || got.X != l.X || got.Y != l.Y || got.Seed != l.Seed ||
				got.MapID != l.MapID || got.ScenarioID != l.ScenarioID || got.History != l.History || got.Archetype != l.Archetype {
				t.Fatalf("%+v: %s differs from the bare galaxy's", opts, id)
			}
		}
		gb, _ := json.Marshal(bare.Galaxy)
		gc, _ := json.Marshal(c.Galaxy)
		if !bytes.Equal(gb, gc) {
			t.Errorf("%+v: the galaxy record differs", opts)
		}
	}
}

// Generation succeeds for every shape and size over seeds 1 to 30: the
// systems are all there and min-separated, every landfall owns a system
// within its safe radius, no two landfalls share a system or a safe start,
// and the gate's controls world lies outside every safe start, deep in the
// galaxy: linked to the landfalls by hops of at most 40, within 15% of the
// farthest any such system lies from its nearest landfall point, and at
// least 0.15 of the radius from every one.
func TestGalaxyLayoutEveryShapeAndSize(t *testing.T) {
	useRepoData(t)
	cfg := genConfig()
	safeR := safeRadius(cfg)
	want := cfg.FactionsMax + 1
	for _, sh := range cfg.Galaxy.Shapes {
		for _, sz := range cfg.Galaxy.Sizes {
			for seed := int64(1); seed <= 30; seed++ {
				lay, err := layoutGalaxy(rand.New(rand.NewSource(seed)), cfg, sz, sh.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(lay.pts) != sz.Systems {
					t.Fatalf("%s/%s seed %d: %d systems, want %d", sh.ID, sz.ID, seed, len(lay.pts), sz.Systems)
				}
				if len(lay.landfalls) < want {
					t.Errorf("%s/%s seed %d: %d landfalls, want at least %d", sh.ID, sz.ID, seed, len(lay.landfalls), want)
				}
				for i, lf := range lay.landfalls {
					p, n := lay.pts[lf.sys], 0
					for _, q := range lay.pts {
						if q != p && math.Hypot(p[0]-q[0], p[1]-q[1]) <= landfallReach+safeR {
							n++
						}
					}
					if n < 2 {
						t.Errorf("%s/%s seed %d: landfall %d's system has %d neighbours", sh.ID, sz.ID, seed, i, n)
					}
				}
				owner := map[int]int{}
				for i, lf := range lay.landfalls {
					p := lay.pts[lf.sys]
					if d := math.Hypot(p[0]-lf.x, p[1]-lf.y); d > safeR+1e-6 {
						t.Errorf("%s/%s seed %d: landfall %d's system is %.1f away, past the safe radius", sh.ID, sz.ID, seed, i, d)
					}
					for j, q := range lay.pts {
						if math.Hypot(q[0]-lf.x, q[1]-lf.y) > safeR+1e-6 {
							continue
						}
						if o, ok := owner[j]; ok {
							t.Errorf("%s/%s seed %d: landfalls %d and %d share a safe system", sh.ID, sz.ID, seed, o, i)
						}
						owner[j] = i
					}
				}
				cp := lay.pts[lay.controls]
				minD := math.Inf(1)
				for _, lf := range lay.landfalls {
					minD = math.Min(minD, math.Hypot(cp[0]-lf.x, cp[1]-lf.y))
				}
				if minD <= safeR+1 {
					t.Errorf("%s/%s seed %d: the controls world is inside a safe start", sh.ID, sz.ID, seed)
				}
				farthest := 0.0
				var lfs []Landfall
				for _, lf := range lay.landfalls {
					lfs = append(lfs, Landfall{X: lf.x, Y: lf.y})
				}
				linked := linkedToLandfalls(lay.pts, lfs, controlsLinkReach)
				if !linked[lay.controls] {
					t.Errorf("%s/%s seed %d: the controls world is cut off from the landfalls", sh.ID, sz.ID, seed)
				}
				for i, p := range lay.pts {
					if !linked[i] {
						continue
					}
					d := math.Inf(1)
					for _, lf := range lay.landfalls {
						d = math.Min(d, math.Hypot(p[0]-lf.x, p[1]-lf.y))
					}
					if d > safeR+1 && i >= 0 {
						farthest = math.Max(farthest, d)
					}
				}
				if minD < controlsSpread*farthest-1e-9 {
					t.Errorf("%s/%s seed %d: the controls world is %.0f from a landfall, the farthest system %.0f", sh.ID, sz.ID, seed, minD, farthest)
				}
				if minD < 0.15*sz.Radius {
					t.Errorf("%s/%s seed %d: the controls world is only %.0f from a landfall (radius %.0f)", sh.ID, sz.ID, seed, minD, sz.Radius)
				}
			}
		}
	}
}

// Every shape and size makes a whole galaxy: each landfall system bound
// safe (never a threat), the gate a site in space beside its controls world
// and outside every safe radius.
func TestGalaxyFullEveryShapeAndSize(t *testing.T) {
	useRepoData(t)
	cfg := genConfig()
	safeR := safeRadius(cfg)
	seeds := []int64{1, 2}
	if testing.Short() {
		seeds = seeds[:1]
	}
	for _, sh := range cfg.Galaxy.Shapes {
		for _, sz := range cfg.Galaxy.Sizes {
			for _, seed := range seeds {
				c, err := generateGalaxy(seed, sz.ID, sh.ID)
				if err != nil {
					t.Fatal(err)
				}
				g := c.Galaxy
				for i, lf := range g.Landfalls {
					loc := c.Locations[lf.System]
					if loc == nil {
						t.Fatalf("%s/%s seed %d: landfall %d has no system", sh.ID, sz.ID, seed, i)
					}
					for _, s := range c.LandfallSafe(i) {
						if tier := c.LocationTier(s); tier == history.TierThreat {
							t.Errorf("%s/%s seed %d: %s in landfall %d's safe start is a threat", sh.ID, sz.ID, seed, s.ID, i)
						}
					}
				}
				gate, ctl := c.GateLocation(), c.Locations[g.ControlsID]
				if gate == nil || gate.Kind != GateKind || gate.HasSite() || ctl == nil || !ctl.HasSite() {
					t.Fatalf("%s/%s seed %d: gate %v, controls %v", sh.ID, sz.ID, seed, gate, ctl)
				}
				if d := math.Hypot(gate.X-ctl.X, gate.Y-ctl.Y); d > cfg.Galaxy.GateHopMax+1e-6 {
					t.Errorf("%s/%s seed %d: the gate is %.1f from its controls world", sh.ID, sz.ID, seed, d)
				}
				for _, l := range []*Location{gate, ctl} {
					if c.nearLandfall(l.X, l.Y, safeR) {
						t.Errorf("%s/%s seed %d: %s is inside a landfall's safe radius", sh.ID, sz.ID, seed, l.ID)
					}
				}
			}
		}
	}
}

// The expedition arrives at a rolled landfall: the Wreck Site on its point,
// the landfall system the nearest destination and a safe one.
func TestExpeditionArrivesAtLandfall(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 8; seed++ {
		c := genCampaign(t, seed)
		lf := c.Galaxy.Landfalls[c.Landfall]
		w := c.Locations[AdriftLocationID]
		if w == nil || w.X != lf.X || w.Y != lf.Y || c.CurrentLocationID != w.ID {
			t.Fatalf("seed %d: the Wreck Site is not at the landfall point", seed)
		}
		start := c.StartLocation()
		if start == nil || start.ID != lf.System || !c.firstJump(start) {
			t.Fatalf("seed %d: start %v is not the landfall's system within a first jump", seed, start)
		}
		for _, l := range c.Systems() {
			if l != start && c.FuelCost(l.ID) < c.FuelCost(start.ID) {
				t.Errorf("seed %d: %s is nearer than the landfall system", seed, l.ID)
			}
		}
	}
}

// Nothing adds a system after generation: scans at every scanner level,
// jumps with their travel rolls, star charts, contracts and 300 faction
// days leave the location count where it was (boarding decks, a faction
// ship's own deck, are not systems).
func TestNoLocationAddedAfterGeneration(t *testing.T) {
	useRepoData(t)
	for _, seed := range []int64{3, 11} {
		c, err := GenerateCampaignOpts("Count", seed, GenOptions{Factions: 3})
		if err != nil {
			t.Fatal(err)
		}
		count := func() int {
			n := 0
			for _, l := range c.Locations {
				if l.Kind != EncounterKind {
					n++
				}
			}
			return n
		}
		before := count()
		systems := c.Systems()
		for i := 0; i < 60; i++ {
			c.CurrentLocationID = systems[(i*7)%len(systems)].ID
			c.Scan(1 + i%3)
			c.MaybeTravelQuests()
			c.RevealCharts(2)
		}
		for _, q := range c.AvailableQuests() {
			c.AcceptQuest(q.ID)
		}
		for day := 1; day <= 300; day++ {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
		}
		if after := count(); after != before {
			t.Errorf("seed %d: %d locations after play, %d at generation", seed, after, before)
		}
	}
}

// A contract points at an existing system of its archetype, never one in a
// safe start.
func TestContractsPointAtExistingSystems(t *testing.T) {
	useRepoData(t)
	_, qt, err := loadGenTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 5; seed++ {
		c := genCampaign(t, seed)
		before := len(c.Locations)
		rng := rand.New(rand.NewSource(seed))
		for i := 0; i < 10; i++ {
			q := c.makeContract(rng, qt)
			if q == nil {
				continue
			}
			c.QuestDefs = append(c.QuestDefs, *q)
			loc := c.Locations[q.Location]
			if loc == nil || !loc.HasSite() || c.safeStart(loc) {
				t.Errorf("seed %d: contract %s points at %v", seed, q.ID, loc)
			}
		}
		if len(c.Locations) != before {
			t.Errorf("seed %d: contracts added locations", seed)
		}
	}
}

func isLandfallSystem(c *Campaign, id string) bool {
	for _, lf := range c.Galaxy.Landfalls {
		if lf.System == id {
			return true
		}
	}
	return false
}

// Every rival arrives at a landfall of its own: a planet-carried rival on
// its landfall system, a ship faction's first ship landed there, never the
// player's landfall and never two on one, by the count, the rival boxes and
// in a spreading race's run.
func TestRivalsArriveAtLandfalls(t *testing.T) {
	useRepoData(t)
	cases := []GenOptions{{Factions: 4}, {Rivals: []RivalPick{{Doctrine: "greyish"}, {Doctrine: "hive"}, {}, {}}}, {Doctrine: "hive", Factions: 3}}
	for _, opts := range cases {
		for seed := int64(1); seed <= 6; seed++ {
			c, err := GenerateCampaignOpts("Arrive", seed, opts)
			if err != nil {
				t.Fatal(err)
			}
			used := map[string]int{}
			for _, l := range c.Locations {
				for _, p := range l.Factions {
					if p.Faction == PlayerFaction {
						continue
					}
					used[l.ID]++
				}
			}
			for _, f := range c.Fleets {
				if !f.Mothership && f.Landed() {
					used[f.LandedID]++
				}
			}
			for id, n := range used {
				if !isLandfallSystem(c, id) || id == c.StartID || n != 1 {
					t.Errorf("%+v seed %d: %d arrivals on %s (landfall %v, start %v)", opts, seed, n, id, isLandfallSystem(c, id), id == c.StartID)
				}
			}
			if len(used) == 0 {
				t.Errorf("%+v seed %d: no rival arrived", opts, seed)
			}
		}
	}
}
