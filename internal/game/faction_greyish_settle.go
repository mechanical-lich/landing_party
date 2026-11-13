package game

import (
	"log"
	"math"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/director"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The greyish as a ship settlement (Phase E4; docs/brainstorm/greyish_plan.md,
// "Materialise and reconcile"). A grey presence is a landed ship: its saucer
// stamped, its hull tier's modules (the vat, the console, from the freighter
// the armory, on the warship the emitters) unpacked by the centre, the
// ground structures it has built (Built, else ShipBuilt) beside the hull,
// its hold in its pocket of lockers, its bodies by type with their gear (the
// record's, else the doctrine's gear floor). Reconcile reads units, ground
// structures, gear, techs and the hold back. When it lifts off the modules
// go aboard, the ground structures become wrecks and the hull goes with the
// ship: the ruin is a landing scar (stampGreyScar).

// isGreyishDoctrine reports a ship settlement doctrine of typed prints (the
// greys).
func isGreyishDoctrine(d *campaign.Doctrine) bool { return d.ShipSettles() }

// greyShip is what a grey presence's ship says about its landing: its hull
// tier, days on the ground and days of its stay left (0 once it is
// leaving: its hold full or its stay over).
type greyShip struct {
	tier, days, stayLeft int
}

// greyShipOf is presence p's ship's landing (all zero without a campaign or
// a landed ship).
func greyShipOf(c *campaign.Campaign, d *campaign.Doctrine, p *campaign.FactionPresence) greyShip {
	if c == nil {
		return greyShip{stayLeft: d.Mobility.StayDays}
	}
	f := c.FleetByPresence(p.ID)
	if f == nil || !f.Landed() {
		return greyShip{stayLeft: d.Mobility.StayDays}
	}
	g := greyShip{tier: f.ShipTier, days: int(c.FactionDay - f.LandedDay)}
	g.stayLeft = max(d.Mobility.StayDays-g.days, 0)
	if c.ShipLeaving(f, c.FactionDay) {
		g.stayLeft = 0
	}
	return g
}

// greyRadius is the director's radius: what the ship counts as its own.
func greyRadius(d *campaign.Doctrine) int { return hiveRadius(d) }

// greyModuleKeys is every structure any hull tier carries, sorted.
func greyModuleKeys(d *campaign.Doctrine) []string {
	seen := map[string]bool{}
	for _, t := range d.ShipTiers() {
		for _, m := range t.Modules {
			seen[m.Key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// greySite is where the k-th of n of key stands round the ship's centre:
// the vat by the centre, the console in the cockpit, the armory beside the
// vat, the array beside the hull, emitters spread round outside the hull.
func greySite(level *world.Level, key string, k, n, cx, cy, cz int) (int, int, int) {
	switch key {
	case "grey_vat":
		return freeLandingTile(level, cx+2, cy+3, cz)
	case "grey_console":
		return freeLandingTile(level, cx+1, cy+1, cz)
	case "grey_armory":
		return freeLandingTile(level, cx-2-k*2, cy+3, cz)
	case "grey_array":
		return freeLandingTile(level, cx+k*2, cy-9, cz)
	case "grey_emitter":
		a := 2*math.Pi*float64(k)/float64(max(n, 1)) + math.Pi/4
		return freeLandingTile(level, cx+int(math.Round(9*math.Cos(a))), cy+int(math.Round(9*math.Sin(a))), cz)
	}
	return freeLandingTile(level, cx, cy+4+k, cz)
}

// greyStanding is the standing structures of key within the ship's radius.
func greyStanding(level *world.Level, d *campaign.Doctrine, key string, cx, cy, cz int) []*ecs.Entity {
	return hiveStructures(level, key, cx, cy, cz, greyRadius(d)+2)
}

// placeGreyStructures stands what is missing of want (key to count, the
// modules and the ground structures together) round the centre, owned by
// the settlement (Kin presence and settlement).
func placeGreyStructures(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, want map[string]int, cx, cy, cz int) {
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !factory.BlueprintExists(key) {
			continue
		}
		for n := len(greyStanding(level, d, key, cx, cy, cz)); n < want[key]; n++ {
			x, y, z := greySite(level, key, n, want[key], cx, cy, cz)
			placeHiveThing(level, key, p.ID, name, x, y, z) // an emitter reads its faction's stance live
		}
	}
}

// greyWant is the structures a ship of g knowing techs stands: its modules
// and its ground structures (the record's Built, else ShipBuilt).
func greyWant(d *campaign.Doctrine, p *campaign.FactionPresence, g greyShip) (want, modules, ground map[string]int) {
	modules = d.ShipModules(g.tier, p.KnownTechs)
	ground = p.Built
	if ground == nil {
		ground = d.ShipBuilt(g.tier, p.KnownTechs, g.days)
	}
	want = map[string]int{}
	for k, v := range modules {
		want[k] += v
	}
	for k, v := range ground {
		want[k] += v
	}
	return want, modules, ground
}

// materialiseGreyish builds a grey ship presence onto level: the saucer
// (once, its cockpit locker the hold), its modules and ground structures,
// its pocket linked and filled (first build), its bodies by type (Phase D
// crew adopted once), and their gear. Idempotent and deterministic from the
// seed and the level.
func materialiseGreyish(c *campaign.Campaign, level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, hostile bool) {
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room for a ship", p.ID, loc.Name)
		return
	}
	name := factionSettlementName(p, loc.ID)
	first := !p.Materialized
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
	if first && !campStands(level, d, name, cx, cy, cz) {
		buildStampedCamp(level, d, p, name, cx, cy, cz)
	}
	want, _, _ := greyWant(d, p, greyShipOf(c, d, p))
	placeGreyStructures(level, d, p, name, want, cx, cy, cz)
	linkPockets(level)
	if first {
		fillFactionStock(level, name, p.Stockpile, cx, cy, cz)
	}
	d.NormalizeUnits(p, loc)
	adoptGreyBodies(level, d, p, name, hostile)
	topUpAutonzBodies(level, d, p, name, cx, cy, cz, hostile)
	handOutGreyGear(level, d, p, name, greyGearWant(d, p))
	p.Materialized = true
}

// campStands reports whether a ship's camp already stands at the centre
// (its settlement owns a container within the camp): a thawed level.
func campStands(level *world.Level, d *campaign.Doctrine, name string, cx, cy, cz int) bool {
	w, h := campSize(d)
	es, scs := levelContainers(level)
	for i, sc := range scs {
		if sc.OwnedBy != name {
			continue
		}
		if pc, ok := entityPos(es[i]); ok && pc.GetZ() == cz && absI(pc.GetX()-cx) <= w/2 && absI(pc.GetY()-cy) <= h/2 {
			return true
		}
	}
	return false
}

// greyTypeOf is a body's type: its Kin type, else (a Phase D body) the
// doctrine's adopt entry, its crew entry's type, the type fallback.
func greyTypeOf(d *campaign.Doctrine, e *ecs.Entity) string {
	if k := components.KinOf(e); k != nil && k.Type != "" {
		return k.Type
	}
	if t, ok := d.Settlement.Adopt[e.Blueprint]; ok {
		return t
	}
	for _, cr := range d.Settlement.Crew {
		if cr.Blueprint == e.Blueprint && cr.Type != "" {
			return cr.Type
		}
	}
	return d.TypeFallback
}

// adoptGreyBodies replaces a Phase D crew body (no Kin type) on its tile by
// its type's print, once (open question 2: the old green miner becomes a
// worker, not a free heavy).
func adoptGreyBodies(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, hostile bool) {
	for _, e := range settlementWorkers(level, name) {
		if k := components.KinOf(e); k != nil && k.Type != "" {
			continue
		}
		typ := greyTypeOf(d, e)
		if typ == "" {
			continue
		}
		pc, _ := entityPos(e)
		x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
		removeBody(level, e)
		spawnAutonzUnit(level, d, p, name, typ, x, y, z, hostile)
	}
}

// greyGearItems is the blueprints a grey body's gear is counted from: the
// outputs of its settlement's recipes that are weapons or armour.
func greyGearItems(d *campaign.Doctrine) map[string]bool {
	out := map[string]bool{}
	for _, k := range d.Settlement.Recipes {
		if r, ok := crafting.GetRecipe(k); ok {
			if it, err := factory.Create(r.Output, 0, 0, 0); err == nil && it.HasComponent(rlcomponents.Item) &&
				(it.HasComponent(rlcomponents.Weapon) || it.HasComponent(rlcomponents.Armor)) {
				out[r.Output] = true
			}
		}
	}
	for _, g := range d.Settlement.GearFloor {
		out[g.Item] = true
	}
	return out
}

// greyGearHeld is the gear the settlement's bodies hold, wear or carry.
func greyGearHeld(level *world.Level, d *campaign.Doctrine, name string) map[string]int {
	gear := greyGearItems(d)
	out := map[string]int{}
	for _, e := range settlementWorkers(level, name) {
		if !e.HasComponent(rlcomponents.Inventory) {
			continue
		}
		inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, it := range append(wornGear(inv), inv.Bag...) {
			if it != nil && gear[it.Blueprint] {
				out[it.Blueprint]++
			}
		}
	}
	return out
}

// greyGearWant is the gear the record says its bodies hold, else the floor.
func greyGearWant(d *campaign.Doctrine, p *campaign.FactionPresence) map[string]int {
	if p.Gear != nil {
		return p.Gear
	}
	return d.GearFloorFor(p.Units, p.KnownTechs)
}

// handOutGreyGear gives the settlement's bodies the gear want holds that
// they do not: each item to a body of a type its floor entry names (in the
// entry's order: soldiers, then heavies), else any fighter, one whose slot
// for it is empty, then equipped best first. Idempotent.
func handOutGreyGear(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, want map[string]int) {
	if len(want) == 0 {
		return
	}
	held := greyGearHeld(level, d, name)
	types := map[string][]string{}
	for _, g := range d.Settlement.GearFloor {
		types[g.Item] = append(types[g.Item], g.Types...)
	}
	bodies := settlementWorkers(level, name)
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, item := range keys {
		for n := held[item]; n < want[item]; n++ {
			proto, err := factory.Create(item, 0, 0, 0)
			if err != nil {
				break
			}
			body := greyGearBody(d, bodies, types[item], proto)
			if body == nil {
				break
			}
			inv := body.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			inv.AddItem(proto)
			inv.EquipAllBest()
		}
	}
}

// greyGearBody is the first body (of types in order, else any fighter:
// soldier, heavy, lancer roles) that does not yet hold an item of it's slot.
func greyGearBody(d *campaign.Doctrine, bodies []*ecs.Entity, types []string, it *ecs.Entity) *ecs.Entity {
	var slot rlcomponents.ItemSlot
	if it.HasComponent(rlcomponents.Item) {
		slot = it.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent).Slot
	}
	if len(types) == 0 {
		for _, k := range d.TypeKeys() {
			if r := d.Types[k].Role; r == "soldier" || r == "heavy" {
				types = append(types, k)
			}
		}
	}
	for _, typ := range types {
		for _, e := range bodies {
			if greyTypeOf(d, e) != typ || !e.HasComponent(rlcomponents.Inventory) {
				continue
			}
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			if slotTaken(inv, slot) {
				continue
			}
			return e
		}
	}
	return nil
}

// slotTaken reports whether inv already has gear (not a natural weapon: an
// item with no value) in slot, its bag counted.
func slotTaken(inv *rlcomponents.InventoryComponent, slot rlcomponents.ItemSlot) bool {
	for _, it := range append(wornGear(inv), inv.Bag...) {
		if it == nil || !it.HasComponent(rlcomponents.Item) {
			continue
		}
		ic := it.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent)
		if ic.Slot == slot && ic.Value > 1 {
			return true
		}
	}
	return false
}

// greyGround is the ground structures standing round the ship: each ground
// key's standing count less what its hull tier carries of it.
func greyGround(level *world.Level, d *campaign.Doctrine, modules map[string]int, cx, cy, cz int) map[string]int {
	out := map[string]int{}
	for _, k := range d.Settlement.Ground {
		if n := len(greyStanding(level, d, k, cx, cy, cz)) - modules[k]; n > 0 {
			out[k] = n
		}
	}
	return out // never nil: an empty map is "nothing standing"
}

// reconcileGreyish writes a grey ship's level back into its record: units
// by type, population, the ground structures standing, the gear its bodies
// hold, the hold in its pocket and its techs. It reports the population.
func reconcileGreyish(c *campaign.Campaign, level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) int {
	name := factionSettlementName(p, loc.ID)
	workers := settlementWorkers(level, name)
	units := map[string]int{}
	for _, e := range workers {
		if t := greyTypeOf(d, e); t != "" {
			units[t]++
		}
	}
	p.Units = units
	if len(p.Units) == 0 {
		p.Units = nil
	}
	p.Population = len(workers)
	if cx, cy, cz, ok := originOf(level, p.ID); ok {
		g := greyShipOf(c, d, p)
		p.Built = greyGround(level, d, d.ShipModules(g.tier, p.KnownTechs), cx, cy, cz)
	}
	p.Gear = greyGearHeld(level, d, name)
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

// reconcileSettlementOf is the right reconcile for a settlement presence:
// the greys', else the generic one.
func reconcileSettlementOf(c *campaign.Campaign, level *world.Level, loc *campaign.Location, p *campaign.FactionPresence) int {
	if d := campaign.LoadFactionData().Doctrine(p.Doctrine); d != nil && isGreyishDoctrine(d) {
		return reconcileGreyish(c, level, loc, d, p)
	}
	return reconcileSettlement(level, loc, p)
}

// greyScarFloor is the floor a departed saucer leaves where its hull stood.
const greyScarFloor = "crater_ground"

// stampGreyScar is a departed grey ship's ruin (greyish_plan.md, "Ruins"):
// the crew and the hold gone with it, its modules packed (the hull tier's
// count of each, the module-only ones all), each ground structure left
// standing as its wreck in place (placed from the ruin's Built when the
// camp was never built), lockers inside the hull gone with it and those
// outside left as plain empty lockers, and the hull itself gone: its walls
// cleared and its floor scorched.
func stampGreyScar(level *world.Level, d *campaign.Doctrine, r *campaign.FactionRuin, p *campaign.FactionPresence, name string, cx, cy, cz int) {
	for _, e := range settlementWorkers(level, name) {
		level.RemoveEntity(e)
	}
	emptySettlementStorage(level, name)
	modules := d.ShipModules(r.Tier, r.Techs)
	for _, key := range greyModuleKeys(d) {
		standing := greyStanding(level, d, key, cx, cy, cz)
		n := modules[key]
		if construction.GetBuildable(key).ModuleOnly || !d.Settlement.IsGround(key) {
			n = len(standing)
		}
		for i := 0; i < n && i < len(standing); i++ {
			level.RemoveEntity(standing[i])
		}
	}
	if r.CampBuilt {
		for _, key := range d.Settlement.Ground {
			for _, e := range greyStanding(level, d, key, cx, cy, cz) {
				wreckInPlace(level, d, e)
			}
		}
	} else {
		built := d.RuinBuilt(r)
		keys := make([]string, 0, len(built))
		for k := range built {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			for i := 0; i < built[key]; i++ {
				if w := d.Settlement.Wrecks[key]; w != "" {
					x, y, z := greySite(level, key, i, built[key], cx, cy, cz)
					if e, err := factory.Create(w, x, y, z); err == nil {
						level.AddEntity(e)
					}
				}
			}
		}
	}
	w, h := campSize(d)
	hull := max(min(w, h)/2-1, 6)
	es, scs := levelContainers(level)
	for i, sc := range scs {
		if sc.OwnedBy != name {
			continue
		}
		pc, ok := entityPos(es[i])
		if ok && pc.GetZ() == cz && math.Hypot(float64(pc.GetX()-cx), float64(pc.GetY()-cy)) <= float64(hull) {
			level.RemoveEntity(es[i]) // the cockpit's locker: part of the hull
			continue
		}
		sc.OwnedBy, sc.Pocket = "", nil
	}
	scarHull(level, cx, cy, cz, hull)
	log.Printf("[faction] %s's ship left %s: a landing scar", p.ID, name)
}

// wreckInPlace replaces a ground structure by its doctrine's wreck on its
// tile (none for a structure without one: it is left).
func wreckInPlace(level *world.Level, d *campaign.Doctrine, e *ecs.Entity) {
	w := d.Settlement.Wrecks[e.Blueprint]
	pc, ok := entityPos(e)
	if w == "" || !ok {
		return
	}
	x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
	level.RemoveEntity(e)
	if wr, err := factory.Create(w, x, y, z); err == nil {
		level.AddEntity(wr)
	}
}

// scarHull clears the saucer's hull walls within r+1 of the centre and
// scorches its floor: the ship took its hull with it.
func scarHull(level *world.Level, cx, cy, cz, r int) {
	for dy := -r - 1; dy <= r+1; dy++ {
		for dx := -r - 1; dx <= r+1; dx++ {
			x, y := cx+dx, cy+dy
			if !level.InBounds(x, y, cz) || math.Hypot(float64(dx), float64(dy)) > float64(r)+0.5 {
				continue
			}
			t := level.GetTilePtr(x, y, cz)
			if t == nil {
				continue
			}
			if !t.Middle.IsEmpty() && world.TileDefinitions[t.Middle.Type].Name == "hull_wall" {
				level.ClearMiddle(x, y, cz)
			}
			level.SetFloor(x, y, cz, greyScarFloor, world.TileVariantAt(greyScarFloor, x, y, cz))
			level.InvalidateSunColumn(x, y)
		}
	}
}

// greyishSiteFields fills a grey ship's director site: the stage's type mix
// among what it knows (the typed grow rung prints the type most under it),
// its types and what it can raise, and its ship: hull tier and stay left.
func greyishSiteFields(s *MainState, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, st *settlement.Settlement, site *director.Site) {
	site.TypeMix = d.TypeMix(p.Stage, loc, st.KnownTechs)
	if site.TypeMix == nil {
		site.TypeMix = map[string]float64{}
	}
	site.Types, site.Unlocked = d.Types, d.TypesUnlocked(st.KnownTechs)
	g := greyShipOf(s.campaign, d, p)
	site.Ship, site.Tier, site.StayLeft = true, g.tier, g.stayLeft
	if d.Ships != nil {
		site.HoldTarget = d.Ships.HoldTarget
	}
	greyDoorWatch(s.level, d, p, st.Name, st.CenterX, st.CenterY, st.CenterZ, s.tick)
	if p.DoorLostTick != 0 {
		// Every door smashed: no locker goes up until the ship re-places its
		// cockpit door a day on (the player's window on the spill).
		site.Structures = map[string]int{"storage_locker": 0}
	}
}

// greyDoorDelay is how long a ship goes without a storage door before it
// re-places its cockpit door: a day (the player's window to carry off a
// spilled pocket; greyish_plan.md, "The storage pocket").
const greyDoorDelay = 750

// greyDoorWatch re-places a ship's cockpit door (an empty storage locker by
// the centre, owned by the settlement) once it has gone greyDoorDelay ticks
// with no storage door standing. The timer is the presence's DoorLostTick.
func greyDoorWatch(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz, tick int) {
	if level == nil || !p.Materialized {
		return
	}
	_, scs := levelContainers(level)
	for _, sc := range scs {
		if sc.OwnedBy == name {
			p.DoorLostTick = 0
			return
		}
	}
	if p.DoorLostTick == 0 {
		p.DoorLostTick = max(tick, 1)
		return
	}
	if tick-p.DoorLostTick < greyDoorDelay {
		return
	}
	x, y, z := freeLandingTile(level, cx-1, cy-1, cz) // the cockpit locker's spot
	if e := placeHiveThing(level, "storage_locker", p.ID, name, x, y, z); e != nil {
		linkPockets(level)
		log.Printf("[faction] %s re-placed its cockpit door at %d,%d,%d", p.ID, x, y, z)
	}
	p.DoorLostTick = 0
}

// boardGreyCrew is a boarded grey ship's crew and defences (greyish_plan.md,
// "Materialise and reconcile", boarding): b.Crew bodies by the ship's units
// (else the stage mix), each its type's body with its stats and the
// boarders' fighting script, holding the ship's gear (else the floor); the
// emitters its hull tier carries; on a mothership as many more as its
// develop boarding entries give (the bulwark).
func boardGreyCrew(level *world.Level, d *campaign.Doctrine, f *campaign.Fleet, b *campaign.Boarding, presenceID string, cx, cy, cz int) {
	techs := f.Presence.KnownTechs
	notePresenceTechs(presenceID, techs)
	units := f.Presence.Units
	if f.Presence.UnitsTotal() != b.Crew {
		tmp := &campaign.FactionPresence{KnownTechs: techs}
		d.AddUnits(tmp, nil, b.Crew)
		units = tmp.Units
	}
	keys := make([]string, 0, len(units))
	for k := range units {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var crew []*ecs.Entity
	i := 0
	for _, typ := range keys {
		dt := d.Types[typ]
		if dt == nil {
			continue
		}
		for n := 0; n < units[typ]; n++ {
			x, y, z := freeLandingTile(level, cx+(i%3)-1, cy+(i/3)%3-1, cz)
			i++
			e, err := factory.Create(dt.Blueprint, x, y, z)
			if err != nil {
				continue
			}
			factory.ApplyType(e, typ, presenceID)
			armBoarder(e, d, f.Presence.Doctrine)
			level.AddEntity(e)
			crew = append(crew, e)
		}
	}
	gear := f.Presence.Gear
	if gear == nil {
		gear = d.GearFloorFor(units, techs)
	}
	gkeys := make([]string, 0, len(gear))
	for k := range gear {
		gkeys = append(gkeys, k)
	}
	sort.Strings(gkeys)
	types := map[string][]string{}
	for _, g := range d.Settlement.GearFloor {
		types[g.Item] = append(types[g.Item], g.Types...)
	}
	for _, item := range gkeys {
		for n := 0; n < gear[item]; n++ {
			it, err := factory.Create(item, 0, 0, 0)
			if err != nil {
				break
			}
			body := greyGearBody(d, crew, types[item], it)
			if body == nil {
				break
			}
			inv := body.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			inv.AddItem(it)
			inv.EquipAllBest()
		}
	}
	emitters := d.ShipModules(f.ShipTier, techs)["grey_emitter"]
	if f.Mothership {
		emitters = 0
		for _, k := range techs {
			if r, ok := research.GetTech(k); ok && r.Develop != nil && r.Develop.Boarding != nil {
				emitters += r.Develop.Boarding.Turrets
			}
		}
	}
	for k := 0; k < emitters; k++ {
		x, y, z := greySite(level, "grey_emitter", k, emitters, cx, cy, cz)
		if e, err := factory.Create("grey_emitter", x, y, z); err == nil {
			level.AddEntity(e) // not the presence's: the crew alone decide the boarding
		}
	}
}
