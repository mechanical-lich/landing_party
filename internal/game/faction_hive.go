package game

import (
	"log"
	"math/rand"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/director"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The hive as a settlement (docs/brainstorm/hive_plan.md). A settled egg (one
// a hive settlement's drone laid, or materialise put down for it) carries its
// settlement on its Kin component. It hatches through the shared egg script;
// its hatchling runs the nursery script instead of hunting, and grows into
// the morph's crew body (xeno_crew, troid_crew): a Worker of that settlement
// with no script, typed by factory.ApplyType. Wild eggs (a scenario's, the
// GameMaster's) age as before.

// nurseryScript is a settled hatchling's script: it stays by where it hatched
// and fights only when attacked.
const nurseryScript = "data/scripts/hive/nursery.basic"

func init() {
	factory.KinStageOverride = settledKinStage
	factory.KinPrepare = prepareSettledKin
	workerai.SettlementDiet = settlementDoctrineDiet
	wireCampaignTechs(nil)
}

// wireCampaignTechs points the bench's requires_tech check at c: a player
// settlement (no "doctrine:" prefix) knows what the campaign knows, a
// faction settlement what its own list holds. nil is a skirmish: every
// settlement reads its own list.
func wireCampaignTechs(c *campaign.Campaign) {
	workerai.KnowsTech = func(name, key string) bool {
		if c != nil && !strings.Contains(name, ":") {
			return c.HasTech(key)
		}
		st, ok := settlement.Settlements[name]
		return !ok || st.HasTech(key)
	}
}

// factionDoctrines maps a faction key that is not itself a doctrine key to
// the doctrine its people run: the observer run's expedition (colony) and a
// race run's player (the chosen race). Set by the world manager from the
// campaign (setFactionDoctrines); every other faction's key is its doctrine's.
var factionDoctrines = map[string]string{}

// setFactionDoctrines points the faction-to-doctrine table at c.
func setFactionDoctrines(c *campaign.Campaign) {
	factionDoctrines = map[string]string{campaign.ExpeditionFaction: campaign.DefaultPlayerDoctrine}
	if c == nil {
		return
	}
	for _, f := range c.Factions() { // every faction instance (rival_selection_plan.md)
		if f.Doctrine != "" {
			factionDoctrines[f.Key] = f.Doctrine
		}
	}
	if c.Player != nil && c.Player.Faction != "" && c.Player.Doctrine != "" {
		factionDoctrines[c.Player.Faction] = c.Player.Doctrine
	}
}

// doctrineOfFaction is the doctrine key a faction's people run.
func doctrineOfFaction(faction string) string {
	if d, ok := factionDoctrines[faction]; ok {
		return d
	}
	return faction
}

// doctrineOfSettlement is the doctrine key of a faction settlement by its
// name ("hive:<loc>"), "" for the player's colony or a name with no faction.
func doctrineOfSettlement(name string) string {
	i := strings.IndexByte(name, ':')
	if i <= 0 {
		return ""
	}
	return doctrineOfFaction(name[:i])
}

// settlementBlock is the settlement block of a faction settlement's doctrine
// by its name ("hive:<loc>"), nil for the player's or an unknown one.
func settlementBlock(name string) *campaign.DoctrineSettlement {
	key := doctrineOfSettlement(name)
	if key == "" {
		return nil
	}
	d := campaign.LoadFactionData().Doctrine(key)
	if d == nil {
		return nil
	}
	return d.Settlement
}

// settlementDoctrineDiet is a faction settlement's diet (the hive eats
// biomass), nil for none.
func settlementDoctrineDiet(name string) []string {
	if s := settlementBlock(name); s != nil {
		return s.Diet
	}
	return nil
}

// grantKillDrops puts the killer's doctrine's kill drops (the hive's
// biomass: every corpse feeds it) into its bag, when a worker of a settlement
// with kill drops killed victim: the worker standing where the victim last
// saw its attacker, else one beside the victim.
func grantKillDrops(level *world.Level, victim *ecs.Entity) {
	pc, ok := entityPos(victim)
	if !ok || victim.HasComponent(rlcomponents.Inanimate) || victim.HasComponent(rlcomponents.Item) {
		return // a structure or a thing, not a corpse
	}
	drops := func(e *ecs.Entity) map[string]int {
		if e == nil || e == victim || !e.HasComponents(components.Worker, components.Settlement) || e.HasComponent(rlcomponents.Dead) {
			return nil
		}
		if s := settlementBlock(e.GetComponent(components.Settlement).(*components.SettlementComponent).Name); s != nil {
			return s.KillDrops
		}
		return nil
	}
	var killer *ecs.Entity
	var buf []*ecs.Entity
	if victim.HasComponent(rlcomponents.AIMemory) {
		m := victim.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		level.GetEntitiesAt(m.AttackerX, m.AttackerY, pc.GetZ(), &buf)
		for _, e := range buf {
			if len(drops(e)) > 0 {
				killer = e
				break
			}
		}
	}
	for dx := -1; dx <= 1 && killer == nil; dx++ {
		for dy := -1; dy <= 1 && killer == nil; dy++ {
			buf = buf[:0]
			level.GetEntitiesAt(pc.GetX()+dx, pc.GetY()+dy, pc.GetZ(), &buf)
			for _, e := range buf {
				if len(drops(e)) > 0 {
					killer = e
					break
				}
			}
		}
	}
	if killer == nil || !killer.HasComponent(rlcomponents.Inventory) {
		return
	}
	inv := killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	keys := make([]string, 0)
	for k := range drops(killer) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, bp := range keys {
		n := drops(killer)[bp]
		it, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			continue
		}
		if it.HasComponent(components.Material) {
			it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = n
			inv.AddItem(it)
			continue
		}
		inv.AddItem(it)
		for i := 1; i < n; i++ {
			if more, err := factory.Create(bp, 0, 0, 0); err == nil {
				inv.AddItem(more)
			}
		}
	}
}

// settledKinStage is a settled body's next stage: its morph's crew body for
// the working adult (stage 2) while its settlement stands; "" otherwise (the
// default, the morph's own stage blueprint).
func settledKinStage(parent *ecs.Entity, stage string) string {
	k := components.KinOf(parent)
	if k == nil || k.Settlement == "" || stage != "stage2" {
		return ""
	}
	if _, ok := settlement.Settlements[k.Settlement]; !ok {
		return ""
	}
	return factory.KinBlueprint(parent.Blueprint, "crew")
}

// prepareSettledKin readies a settled body before its type is applied: a
// hatchling gets the nursery script, a crew adult its settlement and worker.
func prepareSettledKin(parent, child *ecs.Entity, stage string) {
	k := components.KinOf(child)
	if k == nil || k.Settlement == "" {
		return
	}
	stampSettlementFaction(child, k.Settlement)
	switch {
	case stage == "hatchling" && child.HasComponent(components.ScriptedAI):
		sc := child.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
		sc.Script = nurseryScript
		if !child.HasComponent(rlcomponents.AIMemory) {
			child.AddComponent(&rlcomponents.AIMemoryComponent{}) // was_attacked
		}
	case strings.HasSuffix(child.Blueprint, "_crew"):
		enrolHiveWorker(child, k.Settlement)
	}
}

// enrolHiveWorker makes e a worker of settlement name (its faction from the
// settlement key, "hive:<loc>").
func enrolHiveWorker(e *ecs.Entity, name string) {
	e.AddComponent(&components.SettlementComponent{Name: name})
	e.AddComponent(&components.WorkerComponent{SelfDefend: true})
	if i := strings.IndexByte(name, ':'); i > 0 && e.HasComponent(rlcomponents.Description) {
		e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = name[:i]
	}
	if k := components.KinOf(e); k != nil {
		k.Settlement = name
	}
}

// hiveWorker reports whether e is a settled hive body that works (a crew
// adult: a Kin worker of a faction settlement, no script).
func hiveWorker(e *ecs.Entity) bool {
	k := components.KinOf(e)
	return k != nil && k.Settlement != "" && e.HasComponent(components.Worker)
}

// ---- the hive on a level (hive_plan.md, "Materialise and reconcile") ----

// resinTile is the hive's floor.
const resinTile = "hive_resin"

// isHiveDoctrine reports whether d runs a typed settlement (the hive).
func isHiveDoctrine(d *campaign.Doctrine) bool { return d.Broods() && d.Settlement != nil }

// hiveBodies is a hive presence's living bodies on level: its typed workers,
// hatchlings and eggs (by Kin presence or settlement), without its
// structures, larvae and shells.
func hiveBodies(level *world.Level, id, name string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range level.Entities {
		if e == nil || e.HasComponent(rlcomponents.Dead) || e.HasComponent(rlcomponents.Inanimate) {
			continue
		}
		k := components.KinOf(e)
		if k == nil || k.Morph == structureMorph || k.Morph == larvaMorph {
			continue
		}
		if (id != "" && k.Presence == id) || (name != "" && k.Settlement == name) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return lessPos(out[i], out[j]) })
	return out
}

// larvaMorph is the Kin morph of a grey egg.
const larvaMorph = "larva"

func lessPos(a, b *ecs.Entity) bool {
	pa, _ := entityPos(a)
	pb, _ := entityPos(b)
	if pa == nil || pb == nil {
		return pa != nil
	}
	if pa.GetY() != pb.GetY() {
		return pa.GetY() < pb.GetY()
	}
	return pa.GetX() < pb.GetX()
}

// hiveLarvae is settlement name's living larvae on level.
func hiveLarvae(level *world.Level, name string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			if k := components.KinOf(e); k != nil && k.Morph == larvaMorph && k.Settlement == name {
				out = append(out, e)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return lessPos(out[i], out[j]) })
	return out
}

// bodyType is a hive body's type: its Kin type, else (a Phase B base-tier
// body) its morph's fallback.
func bodyType(d *campaign.Doctrine, e *ecs.Entity) string {
	k := components.KinOf(e)
	if k.Type != "" {
		return k.Type
	}
	return d.FallbackType(k.Morph)
}

// hiveStructures is the standing entities of blueprint bp within r of the
// centre on its level (its own: the director counts the same way).
func hiveStructures(level *world.Level, bp string, cx, cy, cz, r int) []*ecs.Entity {
	var out []*ecs.Entity
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.Blueprint != bp || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			if pc, ok := entityPos(e); ok && pc.GetZ() == cz && absI(pc.GetX()-cx) <= r && absI(pc.GetY()-cy) <= r {
				out = append(out, e)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return lessPos(out[i], out[j]) })
	return out
}

// hiveRadius is the director's radius (what the hive counts as its own).
func hiveRadius(d *campaign.Doctrine) int {
	if dir := d.Settlement.Director; dir != nil && dir.Radius > 0 {
		return dir.Radius
	}
	if d.Conversion.Radius > 0 {
		return d.Conversion.Radius
	}
	return 14
}

// materialiseHive builds a hive presence onto level as a settlement: the
// staged camp (heart, pool, the stage's stamp laid with resin), resin out to
// its share, the stage's structures on resin, the stock into its pools, its
// bodies by type (Phase B's scripted bodies adopted), and its larvae.
// Idempotent and deterministic from the presence's seed and the level.
func materialiseHive(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, hostile bool) {
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room for a hive", p.ID, loc.Name)
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
	if first || p.LevelStage < stage {
		buildHiveCamp(level, d, p, name, stage, cx, cy, cz)
	}
	applyPresenceConversion(level, loc, d, p) // resin before the structures that need it
	placeHiveStructures(level, d, p, name, stage, cx, cy, cz)
	if first {
		fillFactionStock(level, name, p.Stockpile, cx, cy, cz)
	}
	if len(p.Units) == 0 && p.Population > 0 {
		// An old save: its units by the composition rule.
		p.Units = map[string]int{}
		for _, b := range presenceComposition(d, loc, stage, p.KnownTechs, p.Population, 0) {
			p.Units[b.typ]++
		}
	}
	d.NormalizeUnits(p, loc)
	adoptHiveBodies(level, d, p, name, hostile)
	topUpHiveBodies(level, loc, d, p, name, cx, cy, cz, hostile)
	topUpLarvae(level, d, p, name, cx, cy, cz)
	p.Materialized = true
}

// buildHiveCamp is the hive's staged camp: at the brood a resin patch round
// the centre; at a stamped stage the stage's chamber (laid with resin),
// stamped over the last with the hive's own things lifted out first and put
// back; then the heart at the centre and a pool when none stands.
func buildHiveCamp(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int) {
	st := d.Stages[stage]
	if st.Stamp != "" {
		w, h := st.W, st.H
		lifted := liftHive(level, p.ID, name, cx-w/2, cy-h/2, w, h, cz)
		stampAt(level, st.Stamp, cx, cy, w, h, nil, p.Seed+int64(stage)*104729)
		for _, e := range lifted {
			pc, _ := entityPos(e)
			x, y, z := freeLandingTile(level, pc.GetX(), pc.GetY(), pc.GetZ())
			pc.SetPosition(x, y, z)
			level.AddEntity(e)
		}
	} else {
		layResinPatch(level, cx, cy, cz, 1)
	}
	camp := d.Settlement.Camp
	if camp != nil && camp.Station != "" && len(hiveStructures(level, camp.Station, cx, cy, cz, 2)) == 0 {
		placeHiveThing(level, camp.Station, p.ID, name, cx, cy, cz)
	}
	if storage := hiveStorage(d); len(hivePools(level, name)) == 0 {
		placeHiveThing(level, storage, p.ID, name, cx+2, cy+1, cz)
	}
	p.LevelStage = stage
}

// hiveStorage is the camp's storage blueprint (the biomass pool).
func hiveStorage(d *campaign.Doctrine) string {
	if c := d.Settlement.Camp; c != nil && c.Storage != "" {
		return c.Storage
	}
	return "storage_locker"
}

// hivePools is settlement name's storage on level.
func hivePools(level *world.Level, name string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range level.Entities {
		if e != nil && e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
			out = append(out, e)
		}
	}
	return out
}

// liftHive takes the hive's own things (structures, larvae, eggs, pools)
// standing in the footprint off the level, for a re-stamp that clears it;
// the caller puts them back.
func liftHive(level *world.Level, id, name string, x0, y0, w, h, cz int) []*ecs.Entity {
	var out []*ecs.Entity
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponents(components.Worker, components.Settlement) {
				continue
			}
			pc, ok := entityPos(e)
			if !ok || pc.GetZ() != cz || pc.GetX() < x0 || pc.GetX() >= x0+w || pc.GetY() < y0 || pc.GetY() >= y0+h {
				continue
			}
			mine := false
			if k := components.KinOf(e); k != nil && (k.Presence == id || k.Settlement == name) {
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

// layResinPatch lays resin on the open ground within r of the centre.
func layResinPatch(level *world.Level, cx, cy, cz, r int) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			x, y := cx+dx, cy+dy
			t := level.GetTilePtr(x, y, cz)
			if t == nil || t.IsSolid() || t.IsWater() || t.Floor.IsEmpty() {
				continue
			}
			level.SetFloor(x, y, cz, resinTile, world.TileVariantAt(resinTile, x, y, cz))
		}
	}
}

// placeHiveThing places blueprint bp of the hive at the free tile nearest
// (x, y, z): its Kin presence and settlement set, a storage claimed.
func placeHiveThing(level *world.Level, bp, id, name string, x, y, z int) *ecs.Entity {
	x, y, z = freeLandingTile(level, x, y, z)
	e, err := factory.Create(bp, x, y, z)
	if err != nil {
		log.Printf("[faction] hive %s: %v", bp, err)
		return nil
	}
	if k := components.KinOf(e); k != nil {
		k.Presence, k.Settlement = id, name
	}
	if e.HasComponent(components.Storage) {
		e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
	}
	stampSettlementFaction(e, name) // a second hive's structures are its own
	level.AddEntity(e)
	return e
}

// hiveSite is the first free resin site of the director's grid round the
// centre (columns two apart, rows three apart, nearest ring first, the ring's
// start from seed), within r: where the director would build.
func hiveSite(level *world.Level, cx, cy, cz, r int, seed int64) (int, int, bool) {
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
			if floorNameAt(level, c[0], c[1], cz) == resinTile && isStandable(level, c[0], c[1], cz) {
				return c[0], c[1], true
			}
		}
	}
	return 0, 0, false
}

// placeHiveStructures stands the stage's structures that are missing (its
// egg clusters, chamber and, at a launcher stage not lost, the spire) and
// the pools its stock needs (one per 100 biomass, at least one), on resin
// sites of the director's grid.
func placeHiveStructures(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int) {
	r := hiveRadius(d)
	want := map[string]int{}
	for k, v := range d.Stages[stage].Structures {
		want[k] = v
	}
	if launcher := d.Level.Launcher; launcher != "" {
		if !d.Stages[stage].Launcher || p.LauncherLost {
			delete(want, launcher)
		} else if want[launcher] == 0 {
			want[launcher] = 1
		}
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, bp := range keys {
		for n := len(hiveStructures(level, bp, cx, cy, cz, r+2)); n < want[bp]; n++ {
			x, y, ok := hiveSite(level, cx, cy, cz, r, p.Seed+int64(i*31+n))
			if !ok {
				log.Printf("[faction] %s: no resin site for %s", p.ID, bp)
				break
			}
			placeHiveThing(level, bp, p.ID, name, x, y, cz)
		}
	}
	pools := max(1, (p.Stockpile["biomass"]+99)/100)
	for n := len(hivePools(level, name)); n < pools; n++ {
		x, y, ok := hiveSite(level, cx, cy, cz, r, p.Seed+int64(97+n))
		if !ok {
			break
		}
		placeHiveThing(level, hiveStorage(d), p.ID, name, x, y, cz)
	}
}

// adoptHiveBodies brings a Phase B presence's scripted bodies into the
// settlement, once: an adult (stage 2 or 3, no Worker) is replaced on its
// tile by its type's crew body; an egg or hatchling is marked the
// settlement's, so it grows into one.
func adoptHiveBodies(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, hostile bool) {
	for _, e := range hiveBodies(level, p.ID, "") {
		k := components.KinOf(e)
		if e.HasComponent(components.Worker) {
			continue
		}
		if strings.HasSuffix(e.Blueprint, "_stage2") || strings.HasSuffix(e.Blueprint, "_stage3") {
			pc, _ := entityPos(e)
			x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
			typ := bodyType(d, e)
			level.RemoveEntity(e)
			spawnHiveCrew(level, d, p, name, typ, x, y, z, hostile)
			continue
		}
		if k.Settlement == "" {
			k.Settlement = name
			if k.Type == "" {
				k.Type = bodyType(d, e)
			}
		}
	}
}

// crewBlueprint is the crew body of type typ's morph ("xeno_crew").
func crewBlueprint(d *campaign.Doctrine, typ string) string {
	if t := d.Types[typ]; t != nil {
		if m := d.Morphs[t.Morph]; m != nil && m.Blueprint != "" {
			return m.Blueprint + "_crew"
		}
	}
	return ""
}

// spawnHiveCrew is a working adult of type typ of the hive at the free tile
// nearest (x, y, z).
func spawnHiveCrew(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name, typ string, x, y, z int, hostile bool) *ecs.Entity {
	bp := crewBlueprint(d, typ)
	if bp == "" {
		return nil
	}
	x, y, z = freeLandingTile(level, x, y, z)
	e, err := spawnFactionWorker(level, bp, name, p.Faction, x, y, z)
	if err != nil {
		log.Printf("[faction] %s crew %s: %v", p.ID, bp, err)
		return nil
	}
	enrolHiveWorker(e, name)
	factory.ApplyType(e, typ, p.ID)
	seedHostility(e, d, hostile)
	return e
}

// topUpHiveBodies tops the level's bodies up (or culls them) to the record's
// units, type by type: a type short gets egg_share of what it lacks as eggs
// on resin by the heart and the rest as working adults; a type over loses
// its eggs first, then its young, then its adults.
func topUpHiveBodies(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz int, hostile bool) {
	have := map[string][]*ecs.Entity{}
	for _, e := range hiveBodies(level, p.ID, name) {
		t := bodyType(d, e)
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
	rng := rand.New(rand.NewSource(p.Seed*31 + int64(len(hiveBodies(level, p.ID, name)))*7919))
	for _, typ := range sorted {
		n := p.Units[typ] - len(have[typ])
		if n < 0 {
			cullBodies(level, have[typ], -n)
			continue
		}
		t := d.Types[typ]
		if t == nil {
			continue
		}
		m := d.Morphs[t.Morph]
		for i := 0; i < n; i++ {
			if share(len(have[typ])+i, d.Level.EggShare) && m != nil {
				x, y, z := hiveNear(level, cx, cy, cz, 3, rng)
				if e, err := factory.CreateTyped(m.Blueprint+"_egg", x, y, z, typ, p.ID); err == nil {
					stampPresenceFaction(e, p)
					components.KinOf(e).Settlement = name
					level.AddEntity(e)
				}
				continue
			}
			r := 7
			if t.Morph != d.DefaultMorph {
				r = 11
			}
			x, y, z := hiveNear(level, cx, cy, cz, r, rng)
			spawnHiveCrew(level, d, p, name, typ, x, y, z, hostile)
		}
	}
}

// hiveNear is a free standable tile within r of the centre, resin first.
func hiveNear(level *world.Level, cx, cy, cz, r int, rng *rand.Rand) (int, int, int) {
	for i := 0; i < 40; i++ {
		x, y := cx+rng.Intn(2*r+1)-r, cy+rng.Intn(2*r+1)-r
		if floorNameAt(level, x, y, cz) == resinTile && isStandable(level, x, y, cz) {
			return x, y, cz
		}
	}
	return freeLandingTile(level, cx+rng.Intn(2*r+1)-r, cy+rng.Intn(2*r+1)-r, cz)
}

// larvaCap is how many larvae settlement name may hold: the larva timer's
// cap plus per_cluster for every standing cluster.
func larvaCap(level *world.Level, d *campaign.Doctrine, cx, cy, cz int) int {
	lv := d.Settlement.Larvae
	if lv == nil {
		return 0
	}
	clusters := 0
	for _, st := range lv.Stations[min(1, len(lv.Stations)):] {
		clusters += len(hiveStructures(level, st, cx, cy, cz, hiveRadius(d)+2))
	}
	return lv.Cap + lv.PerCluster*clusters
}

// larvaStations is the hive's larva stations standing within its radius,
// the heart first.
func larvaStations(level *world.Level, d *campaign.Doctrine, cx, cy, cz int) []*ecs.Entity {
	var out []*ecs.Entity
	for _, st := range d.Settlement.Larvae.Stations {
		out = append(out, hiveStructures(level, st, cx, cy, cz, hiveRadius(d)+2)...)
	}
	return out
}

// spawnLarva lays a grey egg on an open resin tile beside station, reports
// whether there was room.
func spawnLarva(level *world.Level, d *campaign.Doctrine, id, name string, station *ecs.Entity) bool {
	pc, ok := entityPos(station)
	if !ok {
		return false
	}
	for r := 1; r <= 2; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(absI(dx), absI(dy)) != r {
					continue
				}
				x, y, z := pc.GetX()+dx, pc.GetY()+dy, pc.GetZ()
				if floorNameAt(level, x, y, z) != resinTile || !isStandable(level, x, y, z) {
					continue
				}
				e, err := factory.Create(d.Settlement.Larvae.Blueprint, x, y, z)
				if err != nil {
					return false
				}
				if k := components.KinOf(e); k != nil {
					k.Presence, k.Settlement = id, name
				}
				level.AddEntity(e)
				return true
			}
		}
	}
	return false
}

// topUpLarvae tops the level's larvae up to the record's (within the cap),
// beside the heart and the clusters in turn.
func topUpLarvae(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz int) {
	if d.Settlement.Larvae == nil {
		return
	}
	want := min(p.Larvae, larvaCap(level, d, cx, cy, cz))
	stations := larvaStations(level, d, cx, cy, cz)
	for n, i := len(hiveLarvae(level, name)), 0; n < want && len(stations) > 0 && i < 4*want; i++ {
		if spawnLarva(level, d, p.ID, name, stations[i%len(stations)]) {
			n++
		}
	}
}

// stepHiveLarvae is the larva timer of a hive settlement on its level, every
// tick: every every_ticks each station (the heart, then the clusters) lays a
// grey egg beside it while the settlement's larvae are under the cap.
// Deterministic per tick.
func stepHiveLarvae(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, st *settlement.Settlement, tick int) {
	lv := d.Settlement.Larvae
	if lv == nil || lv.EveryTicks <= 0 || tick%lv.EveryTicks != 0 {
		return
	}
	cx, cy, cz := st.CenterX, st.CenterY, st.CenterZ
	n, cap := len(hiveLarvae(level, st.Name)), larvaCap(level, d, cx, cy, cz)
	for _, station := range larvaStations(level, d, cx, cy, cz) {
		if n >= cap {
			return
		}
		if spawnLarva(level, d, p.ID, st.Name, station) {
			n++
		}
	}
}

// reconcileHive writes a hive's level back into its record: units by type
// (eggs and young as their type), population, larvae, the stock in its
// pools and its techs. It reports the population.
func reconcileHive(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) int {
	name := factionSettlementName(p, loc.ID)
	bodies := hiveBodies(level, p.ID, name)
	units := map[string]int{}
	for _, e := range bodies {
		if t := bodyType(d, e); t != "" {
			units[t]++
		}
	}
	p.Units = units
	if len(p.Units) == 0 {
		p.Units = nil
	}
	p.Population = len(bodies)
	p.Larvae = len(hiveLarvae(level, name))
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

// hiveLauncher is a hive's standing spire: one of the presence's (placed by
// materialise) or one its drones built within its radius.
func hiveLauncher(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence) *ecs.Entity {
	if d.Level.Launcher == "" {
		return nil
	}
	if cx, cy, cz, ok := generation.Origin(level, factionOriginKey(p.ID)); ok {
		if ls := hiveStructures(level, d.Level.Launcher, cx, cy, cz, hiveRadius(d)+2); len(ls) > 0 {
			return ls[0]
		}
	}
	return nil
}

// stepLiveHive is the hive's daily live rule on a loaded level: the live
// counts become the record (reconcile), and a stage it has reached has its
// chamber stamped at once (the autonz' rule); its structures are the
// director's to build.
func stepLiveHive(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	if c == nil || loc == nil || level == nil {
		return
	}
	fd := campaign.LoadFactionData()
	ids := make([]string, 0, len(loc.Factions))
	for _, p := range loc.Factions {
		if d := fd.Doctrine(p.Doctrine); d != nil && isHiveDoctrine(d) && p.Materialized && !p.Dormant {
			ids = append(ids, p.ID)
		}
	}
	for _, id := range ids {
		p := loc.PresenceByID(id)
		if p == nil {
			continue
		}
		d := fd.Doctrine(p.Doctrine)
		pop := reconcileHive(level, loc, d, p)
		reconcileConversion(level, d, p)
		if c.ReconcilePresence(loc, id, pop, hiveLauncher(level, d, p) != nil) {
			continue // wiped
		}
		if p.LevelStage < p.Stage {
			// Only the stage's chamber is stamped: on a live level the
			// director builds the stage's structures, nothing is placed.
			if cx, cy, cz, ok := generation.Origin(level, factionOriginKey(p.ID)); ok {
				buildHiveCamp(level, d, p, factionSettlementName(p, loc.ID), min(p.Stage, len(d.Stages)-1), cx, cy, cz)
			}
		}
	}
}

// hiveSiteFields fills a hive director's site: the stage's type mix among
// what its settlement knows, the stage's structure counts, and whether the
// launcher would fire today with the bodies it has (the launch rung walks the
// shot to the spire then).
func hiveSiteFields(s *MainState, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, st *settlement.Settlement, site *director.Site) {
	site.TypeMix = d.TypeMix(p.Stage, loc, st.KnownTechs)
	if site.TypeMix == nil {
		site.TypeMix = map[string]float64{}
	}
	if p.Stage >= 0 && p.Stage < len(d.Stages) {
		site.Structures = d.Stages[p.Stage].Structures
		if site.Structures == nil {
			site.Structures = map[string]int{}
		}
	}
	site.Launcher, site.ShotPop, site.Economy = d.Level.Launcher, d.Mobility.ShotPop, d.EconomyTypes()
	if s.campaign != nil && p.Stage >= 0 && p.Stage < len(d.Stages) && d.Stages[p.Stage].Launcher {
		var loaded func(string) bool
		if s.wm != nil {
			loaded = s.wm.IsLoaded
		}
		pop := len(hiveBodies(s.level, p.ID, st.Name))
		site.LaunchReady = s.campaign.LaunchReady(loc, p.ID, pop, loaded)
	}
}

// launchWalk is how near the spire a hive's shot must stand for the live
// rule to take it (the launch rung's walk; 3 by default).
func launchWalk(d *campaign.Doctrine) int {
	if dir := d.Settlement.Director; dir != nil {
		for _, pr := range dir.Priorities {
			if pr.Launch != nil && pr.Launch.Walk > 0 {
				return pr.Launch.Walk
			}
		}
	}
	return 3
}

// atSpire is a hive's workers holding by its spire (within its launch walk),
// nearest first: the shot its launch rung walked there.
func atSpire(level *world.Level, d *campaign.Doctrine, name string, spire *ecs.Entity) []*ecs.Entity {
	sp, ok := entityPos(spire)
	if !ok {
		return nil
	}
	walk := launchWalk(d)
	var out []*ecs.Entity
	for _, e := range settlementWorkers(level, name) {
		// Only the shot the launch rung walked there and holds, not a drone
		// at work beside the spire (the chamber stands two tiles off).
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask == nil || wc.CurrentTask.Completed || wc.CurrentTask.Data != task_requests.HoldHere {
			continue
		}
		if pc, ok := entityPos(e); ok && pc.GetZ() == sp.GetZ() && absI(pc.GetX()-sp.GetX()) <= walk && absI(pc.GetY()-sp.GetY()) <= walk {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := entityPos(out[i])
		b, _ := entityPos(out[j])
		da := absI(a.GetX()-sp.GetX()) + absI(a.GetY()-sp.GetY())
		db := absI(b.GetX()-sp.GetX()) + absI(b.GetY()-sp.GetY())
		return da < db
	})
	return out
}

// stepHiveBloom grows the hive's farm (hive_plan.md open question 7, as
// built): every every_ticks a resin bloom on an open resin tile within its
// radius, while fewer than one per per_tiles resin tiles stand there. Its
// drones chop it as any flora (two biomass a bloom). Deterministic per tick
// and seed.
func stepHiveBloom(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, st *settlement.Settlement, tick int) {
	b := d.Settlement.Bloom
	if b == nil || b.EveryTicks <= 0 || b.PerTiles <= 0 || tick%b.EveryTicks != 0 {
		return
	}
	cx, cy, cz, r := st.CenterX, st.CenterY, st.CenterZ, hiveRadius(d)
	var open [][2]int
	resin := 0
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			x, y := cx+dx, cy+dy
			if dx*dx+dy*dy > r*r || floorNameAt(level, x, y, cz) != resinTile {
				continue
			}
			resin++
			if isStandable(level, x, y, cz) {
				open = append(open, [2]int{x, y})
			}
		}
	}
	if len(hiveStructures(level, b.Blueprint, cx, cy, cz, r)) >= resin/b.PerTiles || len(open) == 0 {
		return
	}
	rng := rand.New(rand.NewSource(p.Seed*7 + int64(tick)))
	t := open[rng.Intn(len(open))]
	if e, err := factory.Create(b.Blueprint, t[0], t[1], cz); err == nil {
		level.AddEntity(e)
	}
}

// removeBody takes a body off the level for good (a shot, a cull), its task
// handed back to the queue first so the work it held is not lost.
func removeBody(level *world.Level, e *ecs.Entity) {
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			if wc.CurrentTask.Data == task_requests.HoldHere {
				wc.CurrentTask.Complete()
			} else {
				wc.CurrentTask.ReQueue()
			}
			wc.CurrentTask = nil
		}
	}
	level.RemoveEntity(e)
}

// stampSettlementFaction gives a settled body its settlement's faction (the
// prefix of "<faction>:<loc>") in place of its blueprint's: a second hive's
// young are that hive's, never the first's (rival_selection_plan.md,
// "Baked blueprint factions"). The player's own race keeps its bodies'
// blueprint faction until they enrol as workers, as before.
func stampSettlementFaction(e *ecs.Entity, settlementName string) {
	i := strings.IndexByte(settlementName, ':')
	if i <= 0 || !e.HasComponent(rlcomponents.Description) {
		return
	}
	f := settlementName[:i]
	if f == campaign.PlayerFaction || f == doctrineOfFaction(f) {
		return // the player's race, or a faction keyed by its doctrine: as before
	}
	e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = f
}

// stampPresenceFaction gives a presence's freshly spawned body the
// presence's faction when that is an extra instance of its doctrine (a
// second hive's eggs): a faction keyed by its doctrine keeps the blueprint's
// as before.
func stampPresenceFaction(e *ecs.Entity, p *campaign.FactionPresence) {
	if p == nil || p.Faction == p.Doctrine || p.Faction == campaign.PlayerFaction || !e.HasComponent(rlcomponents.Description) {
		return
	}
	e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = p.Faction
}
