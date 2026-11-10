package game

import (
	"log"
	"math"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// Faction presences on a level (docs/developer/factions.md, Tier 2). A
// presence is built onto its site's level when the level is built or thawed
// (materialiseFactions, after the history's evidence and the quest fixtures),
// and counted back into the record when the level freezes (reconcile). The
// presence's creatures carry its ID on their Kin component; scenario and
// GameMaster creatures carry none and are never counted.

// factionOriginKey is the level origin flag holding a presence's centre.
func factionOriginKey(presenceID string) string { return "faction_" + presenceID }

// presenceSiteRadius bounds how far from a hive world's chapter origin a
// presence builds.
const presenceSiteRadius = 30

// materialiseFactions builds every presence on loc into level: the stage's
// stamp, the launcher at a launcher stage, and bodies topped up (or culled)
// to the record's population, at the presence's tier. Deterministic from each
// presence's Seed and what the level already holds; idempotent: a presence
// whose stamp and bodies are all there adds nothing. sc is the site's
// scenario (its egg spawn rules place eggs), nil for none.
func materialiseFactions(c *campaign.Campaign, loc *campaign.Location, level *world.Level, sc *scenario.Scenario) {
	if loc == nil || level == nil {
		return
	}
	// Camps left behind first: a ship back on the same site builds anew.
	stampRuins(c, loc, level)
	fd := campaign.LoadFactionData()
	// Dormant presences are a sleeper node only; a woken one's node goes
	// (phase E).
	for i := range loc.Factions {
		p := &loc.Factions[i]
		if d := fd.Doctrine(p.Doctrine); d != nil && d.Level.Sleeper != "" {
			if p.Dormant {
				placeSleeper(level, loc, d, p)
			} else {
				removeSleeper(level, d, p.ID)
			}
		}
	}
	ps, ds := settlementPresences(c, loc)
	for i, p := range ps {
		materialiseSettlement(level, loc, ds[i], p, c.Stance(p.Faction) == campaign.StanceHostile)
		applyPresenceConversion(level, loc, ds[i], p)
	}
	for i := range loc.Factions {
		p := &loc.Factions[i]
		if d := fd.Doctrine(p.Doctrine); p.Dormant || d != nil && d.Settlement != nil {
			continue // a rival landing party: materialiseSettlements
		} else if d != nil && len(d.Stages) > 0 {
			materialisePresence(level, loc, d, p, sc)
			applyPresenceConversion(level, loc, d, p)
		}
	}
	for i := range loc.Husks {
		if d := fd.Doctrine(loc.Husks[i].Doctrine); d != nil {
			stampHusk(level, loc, d, &loc.Husks[i])
		}
	}
}

func materialisePresence(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, sc *scenario.Scenario) {
	stage := min(max(p.Stage, 0), len(d.Stages)-1)
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room on the level", p.ID, loc.Name)
		return
	}
	notePresenceTechs(p.ID, p.KnownTechs)
	if !p.Materialized || p.LevelStage < stage {
		if st := d.Stages[stage]; st.Stamp != "" {
			stampAt(level, st.Stamp, cx, cy, st.W, st.H, nil, p.Seed+int64(stage)*104729)
		}
		p.LevelStage = stage
	}
	if d.Stages[stage].Launcher && !p.LauncherLost && d.Level.Launcher != "" && presenceLauncher(level, p.ID) == nil {
		x, y, z := freeLandingTile(level, cx, cy, cz)
		// A structure keeps its own sprite: types colour creatures only.
		if e, err := factory.Create(d.Level.Launcher, x, y, z); err == nil {
			components.KinOf(e).Presence = p.ID
			level.AddEntity(e)
		}
	}
	live := presenceBodies(level, p.ID)
	switch n := p.Population - len(live); {
	case n > 0:
		rng := rand.New(rand.NewSource(p.Seed*31 + int64(len(live))*7919 + int64(stage)))
		spawnPresenceBodies(level, loc, d, p, stage, n, len(live), cx, cy, cz, rng, sc)
	case n < 0:
		cullBodies(level, live, -n)
	}
	p.Materialized = true
}

// presenceCentre is where a presence builds: its saved origin flag, else (a
// first build) a site picked from its Seed that keeps the hazard keep-out from
// the landing and clear of the history's stamps, by the chapter's origin on a
// hive world. The pick is saved on the level as an origin flag.
func presenceCentre(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) (int, int, int, bool) {
	key := factionOriginKey(p.ID)
	if x, y, z, ok := generation.Origin(level, key); ok {
		return x, y, z, true
	}
	w, h := 0, 0
	for _, st := range d.Stages {
		w, h = max(w, st.W), max(h, st.H)
	}
	w, h = max(w, 9), max(h, 9)
	rng := rand.New(rand.NewSource(p.Seed))
	keep := generation.Budget().HazardKeepOut
	nx, ny := -1, -1
	if d.IsHome(loc.ScenarioID) {
		if ox, oy, _, ok := generation.Origin(level, generation.ChapterOrigin); ok {
			nx, ny = ox, oy
		}
	}
	// The player's base is no stamp, but the site keeps clear of it too: a
	// footprint over what the colony owns, for the siting only (a stamp's
	// ClearFootprint would take its structures away).
	n0 := len(level.StampFootprints)
	if fp, ok := colonyFootprint(level, loc.SettlementName); ok {
		level.StampFootprints = append(level.StampFootprints, fp)
	}
	x, y, z, ok := generation.FactionSiteNear(level, rng, w, h, keep, nx, ny, presenceSiteRadius)
	level.StampFootprints = level.StampFootprints[:n0]
	if !ok {
		return 0, 0, 0, false
	}
	generation.SetOrigin(level, key, x, y, z)
	return x, y, z, true
}

// stampAt builds a structure script centred on (cx, cy), clearing the
// footprint first, and records the footprint so later stamps keep clear.
func stampAt(level *world.Level, script string, cx, cy, w, h int, params map[string]any, seed int64) {
	if w <= 0 {
		w = 9
	}
	if h <= 0 {
		h = 9
	}
	x0 := min(max(cx-w/2, 0), max(level.GetWidth()-w, 0))
	y0 := min(max(cy-h/2, 0), max(level.GetHeight()-h, 0))
	ctx := &setupContext{Level: level, rng: rand.New(rand.NewSource(seed))}
	top := ctx.topParams()
	for k, v := range params {
		top[k] = v
	}
	generation.ClearFootprint(level, x0, y0, w, h)
	if err := runGenStructure(ctx, script, x0, y0, w, h); err != nil {
		log.Printf("[faction] stamp %s: %v", script, err)
		return
	}
	level.StampFootprints = append(level.StampFootprints, [6]int{x0, y0, w, h, cx, cy})
}

// presenceMembers are the living entities of presence id on level: its
// creatures and eggs, and its structures (the launcher).
func presenceMembers(level *world.Level, id string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range level.Entities {
		if k := components.KinOf(e); k != nil && k.Presence == id && !e.HasComponent(rlcomponents.Dead) {
			out = append(out, e)
		}
	}
	return out
}

// presenceBodies are presence id's creatures and eggs on level (its
// population), without its structures.
func presenceBodies(level *world.Level, id string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range presenceMembers(level, id) {
		if m := components.KinOf(e).Morph; m != structureMorph && m != larvaMorph && !e.HasComponent(rlcomponents.Inanimate) {
			out = append(out, e) // not its structures, larvae or shells
		}
	}
	return out
}

// structureMorph is the Kin morph of a faction's structures (the launcher).
const structureMorph = "structure"

// presenceLauncher is presence id's standing launcher on level (a structure
// of its that is a doctrine's launcher, not its heart or clusters), or nil.
func presenceLauncher(level *world.Level, id string) *ecs.Entity {
	launchers := map[string]bool{}
	for _, d := range campaign.LoadFactionData().Doctrines {
		if d.Level.Launcher != "" {
			launchers[d.Level.Launcher] = true
		}
	}
	for _, e := range presenceMembers(level, id) {
		if components.KinOf(e).Morph == structureMorph && launchers[e.Blueprint] {
			return e
		}
	}
	return nil
}

// bodyKind is one body to spawn: a unit type, its morph and a life stage.
type bodyKind struct{ typ, morph, stage string }

// presenceComposition splits n bodies of a stage-stage presence on loc
// knowing techs: EggShare eggs, AdultShare of the rest adults (stage 3), the
// rest stage 2, each kind spread over the stage's mix by type (locked types'
// shares to their morph's fallback; doctrine_types.go). offset is how many
// bodies the level already holds, so topping up continues the same pattern.
// Deterministic: no randomness.
func presenceComposition(d *campaign.Doctrine, loc *campaign.Location, stage int, techs []string, n, offset int) []bodyKind {
	mix := d.TypeMix(stage, loc, techs)
	types := make([]string, 0, len(mix))
	for k := range mix {
		if t := d.Types[k]; t != nil && (d.Morphs[t.Morph] != nil || !d.Broods()) {
			types = append(types, k)
		}
	}
	sort.Strings(types)
	if len(types) == 0 {
		fb := d.FallbackType(d.DefaultMorph)
		if d.Types[fb] == nil {
			return nil
		}
		types = []string{fb}
		mix = map[string]float64{fb: 1}
	}
	total := 0.0
	for _, t := range types {
		total += mix[t]
	}
	// Kind of the k-th body of the presence overall (0-based): eggs and
	// hatched by share, type by the mix, laid out evenly so any prefix of
	// the sequence keeps the proportions.
	kindAt := func(k int) bodyKind {
		stg := "stage2"
		if share(k, d.Level.EggShare) {
			stg = "egg"
		} else if share(k*7+3, d.Level.AdultShare) {
			stg = "stage3"
		}
		acc, pick := 0.0, types[0]
		f := frac(float64(k)*0.6180339887 + 0.5)
		for _, t := range types {
			acc += mix[t] / total
			if f < acc {
				pick = t
				break
			}
		}
		if !d.Broods() {
			stg = "adult" // a print has no life stages
		}
		return bodyKind{pick, d.Types[pick].Morph, stg}
	}
	out := make([]bodyKind, n)
	for i := range out {
		out[i] = kindAt(offset + i)
	}
	return out
}

// share reports whether the k-th item falls in a share s of an evenly spread
// sequence (the golden-ratio low-discrepancy pattern).
func share(k int, s float64) bool { return frac(float64(k)*0.6180339887+0.25) < s }

func frac(v float64) float64 { return v - math.Floor(v) }

// spawnPresenceBodies spawns n typed bodies of presence p around its centre:
// eggs by the scenario's egg spawn rule for their morph when it has one, else
// in the nest; crawlers close in, fliers in a wider ring (guards).
func spawnPresenceBodies(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, stage, n, offset, cx, cy, cz int, rng *rand.Rand, sc *scenario.Scenario) {
	for _, b := range presenceComposition(d, loc, stage, p.KnownTechs, n, offset) {
		m := d.Morphs[b.morph]
		if m == nil || m.Blueprint == "" {
			continue
		}
		bp := m.Blueprint + "_" + b.stage
		var x, y, z int
		placed := false
		if b.stage == "egg" && sc != nil {
			if rule, ok := sc.SpawnRules[bp]; ok {
				x, y, z, placed = ruleTile(level, rule, cx, cy, rng)
			}
		}
		if !placed {
			r := 3
			if b.stage != "egg" {
				r = 7
				if b.morph != d.DefaultMorph {
					r = 11
				}
			}
			tx := cx + rng.Intn(2*r+1) - r
			ty := cy + rng.Intn(2*r+1) - r
			tz := cz
			if sz := level.GetSurfaceZ(tx, ty); sz >= 0 && absI(sz-cz) <= 2 {
				tz = sz
			}
			x, y, z = freeLandingTile(level, tx, ty, tz)
		}
		e, err := factory.CreateTyped(bp, x, y, z, b.typ, p.ID)
		if err != nil {
			continue
		}
		level.AddEntity(e)
	}
}

// ruleTile finds a free tile for a spawn rule near (cx, cy): its tile names
// and its depth band below the surface.
func ruleTile(level *world.Level, rule scenario.SpawnRule, cx, cy int, rng *rand.Rand) (int, int, int, bool) {
	tiles := map[string]bool{}
	for _, t := range rule.Tiles {
		tiles[t] = true
	}
	lo, hi := 0, level.GetDepth()-1
	if rule.MinZDelta != nil {
		lo = level.SurfaceZ + *rule.MinZDelta
	}
	if rule.MaxZDelta != nil {
		hi = level.SurfaceZ + *rule.MaxZDelta
	}
	lo, hi = max(lo, 0), min(hi, level.GetDepth()-1)
	if lo > hi {
		return 0, 0, 0, false
	}
	const r = 16
	for i := 0; i < 160; i++ {
		x, y := cx+rng.Intn(2*r+1)-r, cy+rng.Intn(2*r+1)-r
		z := lo + rng.Intn(hi-lo+1)
		t := level.GetTilePtr(x, y, z)
		if t == nil || t.IsSolid() || t.IsWater() || level.GetEntityAt(x, y, z) != nil || !isStandable(level, x, y, z) {
			continue
		}
		name := ""
		if !t.Floor.IsEmpty() {
			name = world.TileIndexToName[t.Floor.Type]
		}
		if len(tiles) == 0 || tiles[name] {
			return x, y, z, true
		}
	}
	return 0, 0, 0, false
}

// cullBodies removes n of bodies, eggs first, then the youngest: the record
// starved below what the level held while it was frozen.
func cullBodies(level *world.Level, bodies []*ecs.Entity, n int) {
	rank := func(e *ecs.Entity) int {
		switch {
		case hasSuffix(e.Blueprint, "_egg"):
			return 0
		case hasSuffix(e.Blueprint, "_hatchling"):
			return 1
		case hasSuffix(e.Blueprint, "_stage2"):
			return 2
		}
		return 3
	}
	sorted := append([]*ecs.Entity(nil), bodies...)
	sort.SliceStable(sorted, func(i, j int) bool { return rank(sorted[i]) < rank(sorted[j]) })
	for i := 0; i < n && i < len(sorted); i++ {
		removeBody(level, sorted[i])
	}
}

func hasSuffix(s, suf string) bool { return len(s) >= len(suf) && s[len(s)-len(suf):] == suf }

// stampHusk leaves an exhausted presence's husk on the level once: its
// remaining creatures die out and the husk stamp (cracked eggs) stands where
// it lived.
func stampHusk(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, h *campaign.FactionHusk) {
	if h.Stamped {
		return
	}
	for _, e := range presenceMembers(level, h.Presence) {
		level.RemoveEntity(e)
	}
	for _, e := range settlementRemains(level, loc, d, h) {
		level.RemoveEntity(e)
	}
	if d.Level.Husk != "" {
		key := factionOriginKey(h.Presence)
		x, y, _, ok := generation.Origin(level, key)
		if !ok {
			rng := rand.New(rand.NewSource(h.Seed))
			x, y, _, ok = generation.FactionSite(level, rng, max(d.Level.HuskW, 9), max(d.Level.HuskH, 9), generation.Budget().HazardKeepOut)
		}
		if ok {
			egg := "xeno_egg"
			if m := d.Morphs[h.Morph]; m != nil && m.Blueprint != "" {
				egg = m.Blueprint + "_egg"
			}
			stampAt(level, d.Level.Husk, x, y, d.Level.HuskW, d.Level.HuskH,
				map[string]any{"occupants": "none", "egg": egg, "eggs": 5.0}, h.Seed)
		}
	}
	h.Stamped = true
}

// reconcileFactions counts level back into every presence on loc (Tier 2):
// living bodies into Population (a stage rises with them), whether the
// launcher stands, and a presence with no bodies left is wiped (dropped, a
// Legends event). Called whenever the level is written (a save or a freeze),
// so a reload materialises against the record the level was saved with.
func reconcileFactions(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	if c == nil || loc == nil || level == nil {
		return
	}
	reconcileDormant(c, loc, level)
	settled := map[string]int{}
	ps, ds := settlementPresences(c, loc)
	launchers := map[string]bool{}
	for i, p := range ps {
		if !p.Materialized {
			continue
		}
		if isHiveDoctrine(ds[i]) {
			settled[p.ID] = reconcileHive(level, loc, ds[i], p)
			launchers[p.ID] = hiveLauncher(level, ds[i], p) != nil
		} else if isAutonzDoctrine(ds[i]) {
			settled[p.ID] = reconcileAutonz(level, loc, ds[i], p)
		} else {
			settled[p.ID] = reconcileSettlement(level, loc, p)
		}
		reconcileConversion(level, ds[i], p)
	}
	fd := campaign.LoadFactionData()
	for i := range loc.Factions {
		if d := fd.Doctrine(loc.Factions[i].Doctrine); d != nil && d.Settlement == nil {
			reconcileConversion(level, d, &loc.Factions[i])
		}
	}
	ids := make([]string, 0, len(loc.Factions))
	for _, p := range loc.Factions {
		if !p.Dormant {
			ids = append(ids, p.ID)
		}
	}
	for _, id := range ids {
		bodies, ok := settled[id]
		if !ok {
			live := presenceBodies(level, id)
			bodies = len(live)
			c.ReconcileUnits(loc, id, presenceUnits(loc, id, live))
		}
		launcher, isHive := launchers[id]
		if !isHive {
			launcher = presenceLauncher(level, id) != nil
		}
		if c.ReconcilePresence(loc, id, bodies, launcher) {
			log.Printf("[faction] %s on %s wiped out", id, loc.Name)
		}
	}
}

// presenceUnits counts a typed presence's bodies by type (eggs as the type
// they will hatch; a body with no type, a Phase B base-tier one, as its
// morph's fallback type).
func presenceUnits(loc *campaign.Location, id string, bodies []*ecs.Entity) map[string]int {
	p := loc.PresenceByID(id)
	if p == nil {
		return nil
	}
	d := campaign.LoadFactionData().Doctrine(p.Doctrine)
	if !d.HasTypes() {
		return nil
	}
	out := map[string]int{}
	for _, e := range bodies {
		k := components.KinOf(e)
		t := k.Type
		if t == "" {
			t = d.FallbackType(k.Morph)
		}
		if t != "" {
			out[t]++
		}
	}
	return out
}

// stepLiveFactions runs the launcher rule for presences on loaded levels
// (Tier 1 skips them): once per star-date day, since the rule and its
// cooldown are counted in days. Levels in ID order.
func (wm *WorldManager) stepLiveFactions() {
	wm.stepLiveShips()
	wm.stepBoardings()
	c := wm.Campaign
	ids := make([]string, 0, len(wm.loaded))
	for id := range wm.loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		loc, ms := c.Locations[id], wm.loaded[id]
		if loc == nil || ms == nil || len(loc.Factions) == 0 {
			continue
		}
		stepLiveHive(c, loc, ms.level)
		for _, a := range stepLiveAutonz(c, loc, ms.level, wm.IsLoaded) {
			if id == wm.currentID {
				message.PostMessage("Alert", a)
			}
		}
		for _, f := range stepLiveFaction(c, loc, ms.level, wm.IsLoaded) {
			if id != wm.currentID {
				continue
			}
			if d := campaign.LoadFactionData().Doctrine(f.Presence.Doctrine); d != nil {
				// Named by morph (hidden identity).
				message.PostMessage("Alert", d.LaunchAlert(loc))
			}
		}
	}
}

// stepLiveFaction fires every presence on a loaded level that is ready, by
// the Tier 1 rule with the level's live count, from its standing launcher;
// each shot's bodies leave the level (eggs first, nearest the launcher). It
// returns the fleets fired.
func stepLiveFaction(c *campaign.Campaign, loc *campaign.Location, level *world.Level, loaded func(string) bool) []*campaign.Fleet {
	if level == nil {
		return nil
	}
	var fired []*campaign.Fleet
	ids := make([]string, 0, len(loc.Factions))
	for _, p := range loc.Factions {
		ids = append(ids, p.ID)
	}
	fd := campaign.LoadFactionData()
	for _, id := range ids {
		bodies := presenceBodies(level, id)
		launcher := presenceLauncher(level, id)
		ready := launcher != nil
		var shot []*ecs.Entity // the hive: the bodies its launch rung walked there
		if p := loc.PresenceByID(id); p != nil {
			if d := fd.Doctrine(p.Doctrine); d != nil && isHiveDoctrine(d) {
				name := factionSettlementName(p, loc.ID)
				bodies = hiveBodies(level, id, name)
				launcher = hiveLauncher(level, d, p)
				if launcher != nil {
					shot = atSpire(level, d, name, launcher)
				}
				// The launch is physical: it fires only with a shot's bodies
				// at the spire; without them it waits a day, the cooldown
				// unspent.
				ready = launcher != nil && len(shot) >= max(d.Mobility.ShotPop, 1)
			}
		}
		f := c.LiveLaunch(loc, id, len(bodies), ready, loaded)
		if f == nil {
			continue
		}
		fired = append(fired, f)
		p := loc.PresenceByID(id)
		if shot != nil {
			for i := 0; i < len(bodies)-p.Population && i < len(shot); i++ {
				removeBody(level, shot[i])
			}
			log.Printf("[faction] %s on %s launched %s at %s", id, loc.Name, f.ID, f.ToID)
			continue
		}
		removeShot(level, bodies, len(bodies)-p.Population, launcher)
		log.Printf("[faction] %s on %s launched %s at %s", id, loc.Name, f.ID, f.ToID)
	}
	return fired
}

// removeShot takes n of bodies off the level for a launch: eggs first, then
// the rest, nearest the launcher first.
func removeShot(level *world.Level, bodies []*ecs.Entity, n int, launcher *ecs.Entity) {
	if n <= 0 {
		return
	}
	lx, ly := 0, 0
	if launcher != nil {
		pc := launcher.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lx, ly = pc.GetX(), pc.GetY()
	}
	key := func(e *ecs.Entity) int {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		d := absI(pc.GetX()-lx) + absI(pc.GetY()-ly)
		if !hasSuffix(e.Blueprint, "_egg") {
			d += 1 << 20
		}
		return d
	}
	sorted := append([]*ecs.Entity(nil), bodies...)
	sort.SliceStable(sorted, func(i, j int) bool { return key(sorted[i]) < key(sorted[j]) })
	for i := 0; i < n && i < len(sorted); i++ {
		level.RemoveEntity(sorted[i])
	}
}

// applyPresenceConversion is a presence's Conversion share as a biome patch
// around its centre (the doctrine's conversion tile and radius), seeded by
// the presence: the abstract mirror of its ConvertActions while frozen.
func applyPresenceConversion(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) {
	cv := d.Conversion
	if cv.Tile == "" || cv.Radius <= 0 || p.Conversion <= 0 {
		return
	}
	if cx, cy, cz, ok := presenceCentre(level, loc, d, p); ok {
		share := p.Conversion
		if inner := campResinRadius(d, p); inner > 0 {
			// The hive's share is its creep beyond its camp: lay the camp's
			// own ground and that share beyond it, nearest first.
			inside, _, total := generation.ConversionCounts(level, cx, cy, cz, cv.Radius, inner, cv.Tile)
			if total > 0 {
				share = math.Min(1, p.Conversion+float64(inside)/float64(total))
			}
		}
		generation.ApplyConversion(level, cx, cy, cz, cv.Radius, cv.Tile, share, p.Seed)
	}
}

// campResinRadius is how far from its centre a typed settlement's camp lays
// its own floor (the hive: the brood's patch, the nest's and hive's resin
// chambers), which its conversion share does not count; 0 for others.
func campResinRadius(d *campaign.Doctrine, p *campaign.FactionPresence) int {
	if !isHiveDoctrine(d) {
		return 0
	}
	st := d.Stages[min(max(p.LevelStage, 0), len(d.Stages)-1)]
	if st.Stamp == "" {
		return 1
	}
	return min(st.W, st.H)/2 - 1
}

// reconcileConversion reads a built presence's converted share back from
// its level.
func reconcileConversion(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence) {
	cv := d.Conversion
	if cv.Tile == "" || cv.Radius <= 0 || !p.Materialized {
		return
	}
	if cx, cy, cz, ok := generation.Origin(level, factionOriginKey(p.ID)); ok {
		if inner := campResinRadius(d, p); inner > 0 {
			if _, beyond, total := generation.ConversionCounts(level, cx, cy, cz, cv.Radius, inner, cv.Tile); total > 0 {
				p.Conversion = float64(beyond) / float64(total)
			}
			return
		}
		p.Conversion = generation.ConversionShare(level, cx, cy, cz, cv.Radius, cv.Tile)
	}
}

// colonyFootprint is the box over what colony owns on level (its workers and
// the containers it owns), as a stamp footprint (x, y, w, h, -1, -1).
func colonyFootprint(level *world.Level, colony string) ([6]int, bool) {
	if colony == "" {
		return [6]int{}, false
	}
	x0, y0, x1, y1, found := 0, 0, 0, 0, false
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil {
				continue
			}
			mine := e.HasComponent(components.Settlement) &&
				e.GetComponent(components.Settlement).(*components.SettlementComponent).Name == colony
			if !mine && e.HasComponent(components.Storage) {
				mine = e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == colony
			}
			pc, ok := entityPos(e)
			if !mine || !ok {
				continue
			}
			x, y := pc.GetX(), pc.GetY()
			if !found {
				x0, y0, x1, y1, found = x, y, x, y, true
				continue
			}
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
		}
	}
	return [6]int{x0, y0, x1 - x0 + 1, y1 - y0 + 1, -1, -1}, found
}

// settlementRemains is what an exhausted settlement presence leaves on its
// level besides its bodies: its structures (Inanimate, in StaticEntities,
// found by blueprint as the director counts them: the heart, the pool, egg
// clusters, the evolution chamber, the launcher, blooms) within its radius
// of its origin, and anything of its settlement's (Kin or storage owner).
// The husk replaces them; none of it stays ownerless.
func settlementRemains(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, h *campaign.FactionHusk) []*ecs.Entity {
	presence := h.Presence
	cx, cy, cz, ok := generation.Origin(level, factionOriginKey(presence))
	if !ok || d.Settlement == nil {
		return nil
	}
	name := factionSettlementName(&campaign.FactionPresence{ID: presence, Faction: h.Faction}, loc.ID)
	keys := map[string]bool{}
	for _, b := range d.Settlement.Buildables {
		keys[b] = true
	}
	for _, st := range d.Stages {
		for k := range st.Structures {
			keys[k] = true
		}
	}
	if c := d.Settlement.Camp; c != nil {
		keys[c.Station], keys[c.Storage] = true, true
	}
	if b := d.Settlement.Bloom; b != nil {
		keys[b.Blueprint] = true
	}
	if d.Level.Launcher != "" {
		keys[d.Level.Launcher] = true
	}
	delete(keys, "")
	r := hiveRadius(d) + 2
	var out []*ecs.Entity
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponents(components.Worker, components.Settlement) && e.GetComponent(components.Settlement).(*components.SettlementComponent).Name != name {
				continue // someone else's worker
			}
			mine := false
			if k := components.KinOf(e); k != nil && (k.Presence == presence || k.Settlement == name) {
				mine = true
			}
			if e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
				mine = true
			}
			if !mine && keys[e.Blueprint] {
				if pc, ok := entityPos(e); ok && pc.GetZ() == cz && absI(pc.GetX()-cx) <= r && absI(pc.GetY()-cy) <= r {
					mine = true
				}
			}
			if mine {
				out = append(out, e)
			}
		}
	}
	return out
}
