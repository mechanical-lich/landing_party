package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/director"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The autonz as a settlement faction (Phase E3; docs/brainstorm/autonz_plan.md,
// "Materialise and reconcile"). An autonz presence is a settlement of typed
// prints (one blueprint per type, steel or bronze at spawn) that builds its
// stage's structures on its wire: the cogitator, the Signal Core, relay and
// defence pylons, the foundry, converter nodes. Materialise lays the staged
// camp, the wire, then the structures the record holds (Built), the stock
// and the bodies by type; reconcile reads them back.

// isAutonzDoctrine reports a structure-building settlement doctrine of
// typed prints (the autonz).
func isAutonzDoctrine(d *campaign.Doctrine) bool { return d.Builds() }

// autonzWire is the floor the autonz' structures stand on.
var autonzWire = []string{"blue_wires", "red_wires"}

// autonzEdge is how far out a materialised relay pylon stands (the deploy
// rung's edge).
const autonzEdge = 8

// autonzCore is the Signal Core's blueprint.
const autonzCore = "signal_core"

// autonzTypeOf is a body's type: its Kin type, else (a Phase E body) the
// type whose blueprint it is.
func autonzTypeOf(d *campaign.Doctrine, e *ecs.Entity) string {
	if k := components.KinOf(e); k != nil && k.Type != "" {
		return k.Type
	}
	for _, key := range d.TypeKeys() {
		if d.Types[key].Blueprint == e.Blueprint {
			return key
		}
	}
	return ""
}

// autonzStructureKeys is every structure any of d's stages holds, sorted.
func autonzStructureKeys(d *campaign.Doctrine) []string {
	seen := map[string]bool{}
	for _, st := range d.Stages {
		for k := range st.Structures {
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// materialiseAutonz builds an autonz presence onto level: the staged camp
// (its stamp, locker and fabricator), its wire to the recorded share, then
// the recorded structures on the wire, the stock into its lockers (a floor
// when the record has none), its bodies by type and tier (Phase E's crew
// adopted once). Idempotent and deterministic from the seed and the level.
func materialiseAutonz(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, hostile bool) {
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room for a settlement", p.ID, loc.Name)
		return
	}
	name := factionSettlementName(p, loc.ID)
	first := !p.Materialized
	stage := min(max(p.Stage, 0), max(len(d.Stages)-1, 0))
	st := settlement.Settlements[name]
	if st == nil {
		st = &settlement.Settlement{Name: name, CenterX: cx, CenterY: cy, CenterZ: cz}
		settlement.Settlements[name] = st
	}
	for _, k := range p.KnownTechs {
		st.UnlockTech(k)
	}
	presenceSettlement[p.ID] = name
	notePresenceTechs(p.ID, p.KnownTechs)
	switch {
	case first:
		buildStagedCamp(level, d, p, name, stage, cx, cy, cz, true)
	case p.LevelStage < stage:
		restampAutonz(level, d, p, name, stage, cx, cy, cz)
	}
	applyPresenceConversion(level, loc, d, p) // the wire before what stands on it
	foundry := placeAutonzStructures(level, d, p, name, stage, cx, cy, cz)
	if first {
		stock := p.Stockpile
		if len(stock) == 0 {
			// The floor (autonz_plan.md, step 4): a fresh node's feedstock.
			stock = map[string]int{"metal_ore": 20, "crystal": 4}
			if foundry {
				stock["ferrite"] = 10
			}
		}
		fillFactionStock(level, name, stock, cx, cy, cz)
	}
	// An old save's empty units are filled by the growth rule (each body to
	// the type most under the stage's mix), as Tier 1 adds them.
	d.NormalizeUnits(p, loc)
	adoptAutonzBodies(level, d, p, name, hostile)
	topUpAutonzBodies(level, d, p, name, cx, cy, cz, hostile)
	p.Materialized = true
}

// restampAutonz stamps a risen stage's chamber over the last with the
// settlement's own things (its structures, lockers with their contents, the
// fabricator) lifted out first and put back at the nearest free tile, then a
// locker and the fabricator if none stands (the hub stamp places the Signal
// Core itself).
func restampAutonz(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int) {
	s := d.Stages[stage]
	w, h := s.W, s.H
	lifted := liftAutonz(level, d, p, name, cx-w/2, cy-h/2, w, h, cz)
	if s.Stamp != "" {
		stampAt(level, s.Stamp, cx, cy, w, h, nil, p.Seed+int64(stage)*104729)
	}
	for _, e := range lifted {
		pc, _ := entityPos(e)
		x, y, z := freeLandingTile(level, pc.GetX(), pc.GetY(), pc.GetZ())
		pc.SetPosition(x, y, z)
		level.AddEntity(e)
	}
	furnishStagedCamp(level, d, p, name, stage, cx, cy, cz)
	p.LevelStage = stage
}

// liftAutonz takes the settlement's things in the footprint off the level:
// its faction's structures, its lockers and the doctrine's buildables.
func liftAutonz(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, x0, y0, w, h, cz int) []*ecs.Entity {
	var out []*ecs.Entity
	mineBP := map[string]bool{}
	for _, b := range d.Settlement.Buildables {
		mineBP[b] = true
	}
	if c := d.Settlement.Camp; c != nil && c.Station != "" {
		mineBP[c.Station] = true
	}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponents(components.Worker, components.Settlement) || e.HasComponent(rlcomponents.Item) {
				continue
			}
			pc, ok := entityPos(e)
			if !ok || pc.GetZ() != cz || pc.GetX() < x0 || pc.GetX() >= x0+w || pc.GetY() < y0 || pc.GetY() >= y0+h {
				continue
			}
			mine := mineBP[e.Blueprint]
			if k := components.KinOf(e); k != nil && (k.Presence == p.ID || k.Settlement == name) {
				mine = true
			}
			if e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
				mine = true
			}
			if mine {
				out = append(out, e)
			}
		}
	}
	for _, e := range out {
		level.RemoveEntity(e)
	}
	return out
}

// autonzRadius is the director's radius (what the settlement counts as its
// own).
func autonzRadius(d *campaign.Doctrine) int { return hiveRadius(d) }

// placeAutonzStructures stands the structures the record holds (Built, else,
// when Built is unknown (nil), the stage's that its techs allow; an empty
// Built is a razed settlement and places nothing) that are missing, on the wire: relay
// pylons at the wire's edge (where the deploy rung puts one), defence pylons
// spread round the edge, the rest on the director's grid nearest the centre.
// It reports whether a foundry stands.
func placeAutonzStructures(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int) bool {
	want := p.Built
	if want == nil {
		want = d.StageBuilt(stage, p.KnownTechs)
	}
	r := autonzRadius(d)
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, bp := range keys {
		if !factory.BlueprintExists(bp) {
			continue
		}
		have := len(hiveStructures(level, bp, cx, cy, cz, r+2))
		for n := have; n < want[bp]; n++ {
			var x, y int
			var ok bool
			switch bp {
			case campaign.PylonKey:
				x, y, ok = autonzEdgeSite(level, cx, cy, cz, r, autonzEdge, -1, n)
			case "defence_pylon":
				x, y, ok = autonzEdgeSite(level, cx, cy, cz, r, autonzEdge, n, want[bp])
			default:
				x, y, ok = autonzSite(level, cx, cy, cz, r, p.Seed+int64(i*31+n))
			}
			if !ok {
				log.Printf("[faction] %s: no wired site for %s", p.ID, bp)
				break
			}
			placeHiveThing(level, bp, p.ID, name, x, y, cz)
		}
	}
	return len(hiveStructures(level, "foundry", cx, cy, cz, r+2)) > 0
}

// onWire reports a standable, empty wired tile.
func onWire(level *world.Level, x, y, z int) bool {
	f := floorNameAt(level, x, y, z)
	if f != autonzWire[0] && f != autonzWire[1] || !isStandable(level, x, y, z) {
		return false
	}
	return level.GetEntityAt(x, y, z) == nil
}

// autonzSite is the first wired site of the director's grid round the
// centre (columns two apart, rows three apart, nearest ring first, the
// ring's start from seed) within r, not the centre itself.
func autonzSite(level *world.Level, cx, cy, cz, r int, seed int64) (int, int, bool) {
	start := int(uint64(seed) % 8)
	for ring := 1; ring*2 <= r; ring++ {
		var cands [][2]int
		for gx := -ring; gx <= ring; gx++ {
			for gy := -ring; gy <= ring; gy++ {
				if max(absI(gx), absI(gy)) == ring {
					cands = append(cands, [2]int{cx + gx*2, cy + gy*3})
				}
			}
		}
		sort.Slice(cands, func(i, j int) bool {
			return cands[i][1] < cands[j][1] || cands[i][1] == cands[j][1] && cands[i][0] < cands[j][0]
		})
		for k := range cands {
			c := cands[(k+start)%len(cands)]
			if onWire(level, c[0], c[1], cz) {
				return c[0], c[1], true
			}
		}
	}
	return 0, 0, false
}

// autonzEdgeSite is a wired tile at least edge out within r: the k-th of n
// evenly spread round the centre (the nearest wired tile at or past edge on
// that bearing), or with n < 0 the farthest wired tile (ties by position).
func autonzEdgeSite(level *world.Level, cx, cy, cz, r, edge, k, n int) (int, int, bool) {
	if n < 0 {
		bx, by, bd := 0, 0, -1
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				d := max(absI(dx), absI(dy))
				if d < edge || d <= bd || !onWire(level, cx+dx, cy+dy, cz) {
					continue
				}
				bx, by, bd = cx+dx, cy+dy, d
			}
		}
		return bx, by, bd >= 0
	}
	a := 2*math.Pi*float64(k)/float64(max(n, 1)) + math.Pi/4
	for d := r; d >= edge; d-- {
		x := cx + int(math.Round(float64(d)*math.Cos(a)))
		y := cy + int(math.Round(float64(d)*math.Sin(a)))
		if onWire(level, x, y, cz) {
			return x, y, true
		}
	}
	return autonzEdgeSite(level, cx, cy, cz, r, edge, -1, -1)
}

// adoptAutonzBodies brings a Phase E presence's crew into its types, once: a
// worker with no Kin type is replaced on its tile by its type's print (the
// new sprite, the type's stats and role).
func adoptAutonzBodies(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, hostile bool) {
	for _, e := range settlementWorkers(level, name) {
		if k := components.KinOf(e); k != nil && k.Type != "" {
			continue
		}
		typ := autonzTypeOf(d, e)
		if typ == "" {
			continue
		}
		pc, _ := entityPos(e)
		x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
		removeBody(level, e)
		spawnAutonzUnit(level, d, p, name, typ, x, y, z, hostile)
	}
}

// spawnAutonzUnit is a print of type typ of the settlement at the free tile
// nearest (x, y, z), at its tier from what the presence knows.
func spawnAutonzUnit(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name, typ string, x, y, z int, hostile bool) *ecs.Entity {
	dt := d.Types[typ]
	if dt == nil || dt.Blueprint == "" {
		return nil
	}
	x, y, z = freeLandingTile(level, x, y, z)
	e, err := spawnFactionWorker(level, dt.Blueprint, name, p.Faction, x, y, z)
	if err != nil {
		log.Printf("[faction] %s print %s: %v", p.ID, dt.Blueprint, err)
		return nil
	}
	factory.ApplyType(e, typ, p.ID)
	seedHostility(e, d, hostile)
	return e
}

// topUpAutonzBodies tops the level's prints up (or culls them) to the
// record's units, type by type, round the centre.
func topUpAutonzBodies(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz int, hostile bool) {
	have := map[string][]*ecs.Entity{}
	for _, e := range settlementWorkers(level, name) {
		t := autonzTypeOf(d, e)
		have[t] = append(have[t], e)
	}
	keys := map[string]bool{}
	for k := range p.Units {
		keys[k] = true
	}
	for k := range have {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	rng := rand.New(rand.NewSource(p.Seed*31 + int64(len(settlementWorkers(level, name)))*7919))
	for _, typ := range sorted {
		n := p.Units[typ] - len(have[typ])
		if n < 0 {
			for _, e := range have[typ][len(have[typ])+n:] {
				removeBody(level, e)
			}
			continue
		}
		for i := 0; i < n; i++ {
			spawnAutonzUnit(level, d, p, name, typ, cx+rng.Intn(7)-3, cy+rng.Intn(7)-3, cz, hostile)
		}
	}
}

// autonzBuilt is the settlement's standing structures within its radius, by
// blueprint, for every structure a stage holds.
func autonzBuilt(level *world.Level, d *campaign.Doctrine, cx, cy, cz int) map[string]int {
	out := map[string]int{}
	for _, k := range autonzStructureKeys(d) {
		if n := len(hiveStructures(level, k, cx, cy, cz, autonzRadius(d)+2)); n > 0 {
			out[k] = n
		}
	}
	return out // never nil: an empty map is "nothing standing", nil "unknown"
}

// reconcileAutonz writes an autonz settlement's level back into its
// record: units by type, population, the structures standing, the stock in
// its lockers and its techs. It reports the population.
func reconcileAutonz(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) int {
	name := factionSettlementName(p, loc.ID)
	workers := settlementWorkers(level, name)
	units := map[string]int{}
	for _, e := range workers {
		if t := autonzTypeOf(d, e); t != "" {
			units[t]++
		}
	}
	p.Units = units
	if len(p.Units) == 0 {
		p.Units = nil
	}
	p.Population = len(workers)
	if cx, cy, cz, ok := originOf(level, p.ID); ok {
		p.Built = autonzBuilt(level, d, cx, cy, cz)
	}
	p.Stockpile = storage.Summarize(storage.LevelProvider{Level: level}, []string{name})
	if len(p.Stockpile) == 0 {
		p.Stockpile = nil
	}
	if st := settlement.Settlements[name]; st != nil {
		for _, k := range st.KnownTechs {
			if !containsStr(p.KnownTechs, k) {
				p.KnownTechs = append(p.KnownTechs, k)
			}
		}
		notePresenceTechs(p.ID, p.KnownTechs)
	}
	return p.Population
}

// autonzCoreOf is the settlement's standing Signal Core, nil without one.
func autonzCoreOf(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence) *ecs.Entity {
	if cx, cy, cz, ok := originOf(level, p.ID); ok {
		if cs := hiveStructures(level, autonzCore, cx, cy, cz, autonzRadius(d)+2); len(cs) > 0 {
			return cs[0]
		}
	}
	return nil
}

// autonzPylons is how many relay pylons stand within the settlement's
// radius.
func autonzPylons(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence) int {
	if cx, cy, cz, ok := originOf(level, p.ID); ok {
		return len(hiveStructures(level, campaign.PylonKey, cx, cy, cz, autonzRadius(d)+2))
	}
	return 0
}

// autonzSiteFields fills an autonz director's site: the stage's type mix
// among what its settlement knows, the stage's structures, its types and
// what it can raise, whether it would signal today (the transmit rung walks
// its bodies to the core then) and the bearing of the nearest site in its
// signal's reach (where it deploys its pylons).
func autonzSiteFields(s *MainState, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, st *settlement.Settlement, site *director.Site) {
	site.TypeMix = d.TypeMix(p.Stage, loc, st.KnownTechs)
	if site.TypeMix == nil {
		site.TypeMix = map[string]float64{}
	}
	site.Structures = map[string]int{}
	if p.Stage >= 0 && p.Stage < len(d.Stages) {
		for k, v := range d.Stages[p.Stage].Structures {
			site.Structures[k] = v
		}
	}
	site.Types, site.Unlocked = d.Types, d.TypesUnlocked(st.KnownTechs)
	site.ShotPop, site.Economy = d.Mobility.ShotPop, d.EconomyTypes()
	if s.campaign == nil {
		return
	}
	var loaded func(string) bool
	if s.wm != nil {
		loaded = s.wm.IsLoaded
	}
	pylons := autonzPylons(s.level, d, p)
	if p.Stage >= 0 && p.Stage < len(d.Stages) && d.Stages[p.Stage].Signal && autonzCoreOf(s.level, d, p) != nil {
		site.LaunchReady = s.campaign.SignalReady(loc, p.ID, len(settlementWorkers(s.level, st.Name)), pylons, loaded)
	}
	site.SignalDir = s.campaign.SignalBearing(loc, p.ID)
}

// stepLiveAutonzSettlement is an awake autonz settlement's day on its loaded
// level: the level into its record; a signal sent once the Signal Core
// stands and the transmit rung's bodies hold at it (those bodies taken off
// the level), its range from the pylons standing; a risen stage's chamber
// stamped (on a live level the director builds; nothing is placed). It
// reports whether it signalled.
func stepLiveAutonzSettlement(c *campaign.Campaign, loc *campaign.Location, level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, loaded func(string) bool) bool {
	name := factionSettlementName(p, loc.ID)
	reconcileAutonz(level, loc, d, p)
	reconcileConversion(level, d, p)
	workers := settlementWorkers(level, name)
	var shot []*ecs.Entity
	if core := autonzCoreOf(level, d, p); core != nil {
		shot = atSpire(level, d, name, core)
	}
	ready := len(shot) >= max(d.Mobility.ShotPop, 1)
	target := c.LiveSignal(loc, p.ID, len(workers), p.Conversion, autonzPylons(level, d, p), ready, loaded)
	if target != nil {
		for i := 0; i < len(workers)-p.Population && i < len(shot); i++ {
			removeBody(level, shot[i])
		}
		log.Printf("[faction] %s on %s signalled %s", p.ID, loc.Name, target.Name)
	}
	if p.LevelStage < p.Stage {
		if cx, cy, cz, ok := originOf(level, p.ID); ok {
			restampAutonz(level, d, p, name, min(p.Stage, len(d.Stages)-1), cx, cy, cz)
		}
	}
	return target != nil
}

// autonzBuildable reports whether key is a structure its settlement may
// build now (the cheat and the panel).
func autonzBuildable(d *campaign.Doctrine, techs []string, key string) bool {
	b := construction.GetBuildable(key)
	return d.Settlement.CanBuild(key) && !b.DeployOnly && (b.RequiredTech == "" || d.KnowsTech(techs, b.RequiredTech))
}

// originOf is presence id's centre on level (its origin flag).
func originOf(level *world.Level, id string) (int, int, int, bool) {
	return generation.Origin(level, factionOriginKey(id))
}

// grantFactionTech gives faction's presence on the site on screen tech, in
// its record and (built) its settlement: the cheat console's "faction tech".
func grantFactionTech(ms *MainState, faction, tech string) (string, error) {
	if ms == nil || ms.campaign == nil || ms.location() == nil {
		return "", fmt.Errorf("needs a campaign site on screen")
	}
	if _, ok := research.GetTech(tech); !ok {
		return "", fmt.Errorf("unknown tech %q", tech)
	}
	loc := ms.location()
	p := loc.Presence(faction)
	if p == nil {
		return "", fmt.Errorf("no %s presence on %s", faction, loc.Name)
	}
	if !containsStr(p.KnownTechs, tech) {
		p.KnownTechs = append(p.KnownTechs, tech)
	}
	notePresenceTechs(p.ID, p.KnownTechs)
	if st := settlement.Settlements[factionSettlementName(p, loc.ID)]; st != nil {
		st.UnlockTech(tech)
	}
	return fmt.Sprintf("%s on %s knows %s", faction, loc.Name, tech), nil
}
