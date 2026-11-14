package game

import (
	"hash/fnv"
	"log"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/director"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// Faction settlements on loaded levels (phase C2; docs/developer/factions.md,
// "Faction settlements"). A presence whose doctrine has a settlement block
// (the mirror colony now; greyish and autonz later) is not a swarm of
// creatures but a rival landing party: when its level is built it gets a
// camp, a settlement record of its own (keyed "<faction>:<locID>", finding
// 5), workers from the doctrine's crew with Worker and that settlement, its
// stockpile in its own lockers, and a director running the doctrine's needs
// ladder through the player's order paths (settlementOrders). Reconcile
// writes population, techs and stockpile back whenever the level is written.
// The settlement is saved with the location (locationSettlements) and taken
// out of the global map on Freeze.

// factionSettlementName is a presence's settlement key on locID.
func factionSettlementName(p *campaign.FactionPresence, locID string) string {
	return p.Faction + ":" + locID
}

// settlementPresences is every presence on loc that runs a settlement:
// planet-carried ones (loc.Factions) and those of ship fleets landed there,
// with their doctrines. Pointers into the campaign, so writes stick.
func settlementPresences(c *campaign.Campaign, loc *campaign.Location) ([]*campaign.FactionPresence, []*campaign.Doctrine) {
	if loc == nil {
		return nil, nil
	}
	fd := campaign.LoadFactionData()
	var ps []*campaign.FactionPresence
	var ds []*campaign.Doctrine
	add := func(p *campaign.FactionPresence) {
		if p.Dormant {
			return // a sleeper node until it wakes (faction_autonz.go)
		}
		if d := fd.Doctrine(p.Doctrine); d != nil && d.Settlement != nil {
			ps = append(ps, p)
			ds = append(ds, d)
		}
	}
	for i := range loc.Factions {
		add(&loc.Factions[i])
	}
	if c != nil {
		for _, f := range c.LandedFleets(loc.ID) {
			add(&f.Presence)
		}
	}
	return ps, ds
}

// materialiseSettlement builds a settlement presence onto level: the camp
// (once), the settlement record (unless a thawed save restored it), workers
// topped up or culled to the record's population, and on the first build
// the stockpile in the camp's locker. Idempotent, deterministic from the
// presence's seed and what the level holds.
func materialiseSettlement(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence, hostile bool) {
	if isHiveDoctrine(d) {
		materialiseHive(level, loc, d, p, hostile) // faction_hive.go
		return
	}
	if isAutonzDoctrine(d) {
		materialiseAutonz(level, loc, d, p, hostile) // faction_autonz_settle.go
		return
	}
	if isGreyishDoctrine(d) {
		materialiseGreyish(nil, level, loc, d, p, hostile) // no campaign: a scout's modules, nothing built
		return
	}
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room for a settlement", p.ID, loc.Name)
		return
	}
	name := factionSettlementName(p, loc.ID)
	first := !p.Materialized
	stage := min(max(p.Stage, 0), max(len(d.Stages)-1, 0))
	if camp := d.Settlement.Camp; camp != nil && camp.Staged && len(d.Stages) > 0 {
		// The autonz: each stage's stamp is the camp (phase E).
		if first || p.LevelStage < stage {
			buildStagedCamp(level, d, p, name, stage, cx, cy, cz, first)
		}
	} else if first {
		if d.Settlement.Camp != nil {
			buildStampedCamp(level, d, p, name, cx, cy, cz)
		} else {
			buildFactionCamp(level, d, p, name, cx, cy, cz)
		}
	}
	st := settlement.Settlements[name]
	if st == nil {
		st = &settlement.Settlement{Name: name, CenterX: cx, CenterY: cy, CenterZ: cz}
		settlement.Settlements[name] = st
	}
	for _, k := range p.KnownTechs {
		st.UnlockTech(k)
	}
	presenceSettlement[p.ID] = name // its bodies spawn with its live techs
	live := settlementWorkers(level, name)
	switch n := p.Population - len(live); {
	case n > 0:
		crew := d.StageSettlementAt(stage, loc, p.KnownTechs).CrewEntries(p.Population)
		for i := len(live); i < p.Population && i < len(crew); i++ {
			x, y, z := freeLandingTile(level, cx, cy, cz)
			if e, err := spawnFactionWorker(level, crew[i].Blueprint, name, p.Faction, x, y, z); err != nil {
				log.Printf("[faction] %s worker %s: %v", p.ID, crew[i].Blueprint, err)
			} else {
				if crew[i].Type != "" && d.HasTypes() {
					factory.ApplyType(e, crew[i].Type, p.ID) // its type's stats, role and tier
				}
				seedHostility(e, d, hostile)
			}
		}
	case n < 0:
		for _, e := range live[:-n] {
			level.RemoveEntity(e)
		}
	}
	if d.Settlement.StoragePocket {
		linkPockets(level) // its lockers are doors onto one pocket
	}
	if first {
		fillFactionStock(level, name, p.Stockpile, cx, cy, cz)
	}
	p.Materialized = true
}

// spawnFactionWorker is spawnCrewMember for a faction: the crew blueprint
// with Worker and the faction settlement, and the faction string, so it is
// never a colony worker and fights by its own faction.
func spawnFactionWorker(level *world.Level, blueprint, settlementName, faction string, x, y, z int) (*ecs.Entity, error) {
	e, err := spawnCrewMember(level, blueprint, settlementName, x, y, z)
	if err != nil {
		return nil, err
	}
	if e.HasComponent(rlcomponents.Description) {
		e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = faction
	}
	return e, nil
}

// settlementWorkers is the living workers of settlement name on level, in a
// stable order (position, then blueprint).
func settlementWorkers(level *world.Level, name string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range level.Entities {
		if components.IsColonyWorker(e, name) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := entityPos(out[i])
		b, _ := entityPos(out[j])
		if a.GetY() != b.GetY() {
			return a.GetY() < b.GetY()
		}
		return a.GetX() < b.GetX()
	})
	return out
}

// buildFactionCamp is a rival landing party's camp, as the player's start
// camp is laid: a bunker room (the doctrine's first stage footprint, at
// least 9x7), an airlock on its south wall owned by the faction, and a
// storage locker and workbench owned by its settlement.
func buildFactionCamp(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz int) {
	w, h := 9, 7
	if len(d.Stages) > 0 {
		w, h = max(d.Stages[0].W, w), max(d.Stages[0].H, h)
	}
	x0, y0 := cx-w/2, cy-h/2
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			x, y := x0+dx, y0+dy
			level.SetFloor(x, y, cz, "bunker_floor", world.TileVariantAt("bunker_floor", x, y, cz))
			if dx == 0 || dy == 0 || dx == w-1 || dy == h-1 {
				level.SetMiddle(x, y, cz, "bunker_wall", world.TileVariantAt("bunker_wall", x, y, cz))
			} else {
				level.ClearMiddle(x, y, cz)
			}
			level.InvalidateSunColumn(x, y)
		}
	}
	// The way out: the airlock, and the ground outside it cleared.
	ax, ay := cx, y0+h-1
	level.ClearMiddle(ax, ay, cz)
	for k := 1; k <= 2; k++ {
		if level.InBounds(ax, ay+k, cz) {
			level.ClearMiddle(ax, ay+k, cz)
		}
	}
	place := func(bp string, x, y int, own func(*ecs.Entity)) {
		e, err := factory.Create(bp, x, y, cz)
		if err != nil {
			log.Printf("[faction] camp %s: %v", bp, err)
			return
		}
		if own != nil {
			own(e)
		}
		level.AddEntity(e)
	}
	place("airlock", ax, ay, func(e *ecs.Entity) {
		if e.HasComponent(rlcomponents.Door) {
			e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).OwnedBy = p.Faction
		}
	})
	place("storage_locker", x0+1, y0+1, func(e *ecs.Entity) {
		if e.HasComponent(components.Storage) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
		}
	})
	place("basic_workbench", x0+w-2, y0+1, nil)
}

// buildStampedCamp is a camp from the doctrine's camp block (phase D; the
// greyish: their landed saucer): the structure stamped at the centre, the
// storage inside it claimed by the settlement (a locker placed when there is
// none), its doors opened to the faction, and the station (the bio-printer)
// by the centre.
func buildStampedCamp(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, cx, cy, cz int) {
	camp := d.Settlement.Camp
	stampAt(level, camp.Stamp, cx, cy, camp.W, camp.H, camp.Params, p.Seed*31+17)
	x0, y0 := cx-camp.W/2, cy-camp.H/2
	claimed := false
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			pc, ok := entityPos(e)
			if !ok || pc.GetZ() != cz || pc.GetX() < x0 || pc.GetX() >= x0+camp.W || pc.GetY() < y0 || pc.GetY() >= y0+camp.H {
				continue
			}
			if e.HasComponent(components.Storage) {
				e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
				claimed = true
			}
			if e.HasComponent(rlcomponents.Door) {
				e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).OwnedBy = p.Faction
			}
		}
	}
	place := func(bp string, own func(*ecs.Entity)) {
		x, y, z := freeLandingTile(level, cx+2, cy+3, cz)
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			log.Printf("[faction] camp %s: %v", bp, err)
			return
		}
		if own != nil {
			own(e)
		}
		level.AddEntity(e)
	}
	if !claimed {
		place("storage_locker", func(e *ecs.Entity) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
		})
	}
	if camp.Station != "" {
		place(camp.Station, nil)
	}
}

// fillFactionStock puts a stockpile into the settlement's storage, as the
// player's kit is beamed down: into its first container, else (a camp with
// no container: its locker razed, or none in its stamp) on the ground by
// the centre (cx, cy, cz), one stack a tile, nearest first.
func fillFactionStock(level *world.Level, name string, stock map[string]int, cx, cy, cz int) {
	if len(stock) == 0 {
		return
	}
	var box *components.StorageComponent
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if box == nil && e != nil && e.HasComponent(components.Storage) {
				if sc := e.GetComponent(components.Storage).(*components.StorageComponent); sc.OwnedBy == name {
					box = sc
				}
			}
		}
	}
	var ground [][2]int
	if box == nil {
		ground = groundNear(level, cx, cy, cz, 6)
	}
	keys := make([]string, 0, len(stock))
	for k := range stock {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, bp := range keys {
		q := stock[bp]
		if q <= 0 {
			continue
		}
		for q > 0 {
			it, err := factory.Create(bp, 0, 0, 0)
			if err != nil {
				break
			}
			if it.HasComponent(components.Material) {
				it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = q
				q = 0
			} else {
				q-- // one item each (rations)
			}
			if box != nil && !box.AddItem(it) {
				// The locker (or its pocket) is full: the rest on the ground.
				box = nil
				ground = groundNear(level, cx, cy, cz, 6)
			}
			switch {
			case box != nil:
			case len(ground) > 0:
				t := ground[0]
				ground = ground[1:]
				if pc, ok := entityPos(it); ok {
					pc.SetPosition(t[0], t[1], cz)
				}
				level.AddEntity(it)
			default:
				log.Printf("[faction] %s: no container and no ground for its %s", name, bp)
			}
		}
	}
}

// groundNear is the standable tiles within r of (cx, cy) on z with nothing
// on them, nearest first (ties by position).
func groundNear(level *world.Level, cx, cy, cz, r int) [][2]int {
	var out [][2]int
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= level.GetWidth() || y >= level.GetHeight() || !isStandable(level, x, y, cz) {
				continue
			}
			out = append(out, [2]int{x, y})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := absI(out[i][0]-cx) + absI(out[i][1]-cy)
		dj := absI(out[j][0]-cx) + absI(out[j][1]-cy)
		return di < dj
	})
	return out
}

// reconcileSettlement writes a settlement presence's level back into its
// record: living workers into Population, the settlement's storage into
// Stockpile, its techs into KnownTechs. It reports the population.
func reconcileSettlement(level *world.Level, loc *campaign.Location, p *campaign.FactionPresence) int {
	name := factionSettlementName(p, loc.ID)
	pop := len(settlementWorkers(level, name))
	p.Population = pop
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
	}
	return pop
}

func containsStr(list []string, k string) bool {
	for _, v := range list {
		if v == k {
			return true
		}
	}
	return false
}

// factionDirectorSeed is a faction settlement's director seed.
func factionDirectorSeed(c *campaign.Campaign, name string, p *campaign.FactionPresence) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	seed := int64(h.Sum64()>>1) ^ p.Seed
	if c != nil {
		seed ^= c.Seed * 7919
	}
	return seed
}

// stepFactionDirectors runs each faction settlement's director on this
// level: the doctrine's table through settlementOrders, at the
// settlement's centre with its own techs.
func (s *MainState) stepFactionDirectors() {
	if s.campaign == nil || s.locationID == "" || s.level == nil {
		return
	}
	loc := s.campaign.Locations[s.locationID]
	ps, ds := settlementPresences(s.campaign, loc)
	for i, p := range ps {
		d := ds[i]
		name := factionSettlementName(p, loc.ID)
		st := settlement.Settlements[name]
		if st == nil || d.Settlement.Director == nil {
			continue
		}
		if s.factionDirectors == nil {
			s.factionDirectors = map[string]*director.Director{}
		}
		dir := s.factionDirectors[name]
		if dir == nil {
			dir = director.New(factionDirectorSeed(s.campaign, name, p))
			s.factionDirectors[name] = dir
		}
		if d.Settlement.StoragePocket && (s.tick%30 == 0 || dir.LastPlan < 0) {
			linkPockets(s.level) // a locker built since is a door too
		}
		raids := dir.Ordered["raids"]
		hostile := s.campaign.Stance(p.Faction) == campaign.StanceHostile
		site := &director.Site{}
		if isHiveDoctrine(d) {
			stepHiveLarvae(s.level, d, p, st, s.tick)
			stepHiveBloom(s.level, d, p, st, s.tick)
			hiveSiteFields(s, loc, d, p, st, site)
		}
		if isAutonzDoctrine(d) {
			autonzSiteFields(s, loc, d, p, st, site)
		}
		if isGreyishDoctrine(d) {
			greyishSiteFields(s, loc, d, p, st, site) // faction_greyish_settle.go
		}
		site.Level, site.Colony = s.level, name
		site.CX, site.CY, site.CZ = st.CenterX, st.CenterY, st.CenterZ
		site.Tasks, site.Known = &st.Tasks, st.KnownTechs
		site.Doctrine = d.StageSettlementAt(p.Stage, loc, st.KnownTechs)
		site.Orders = settlementOrders{st: st, level: s.level, ds: d.Settlement}
		site.Hostile, site.Enemy, site.Aggression = hostile, s.colonyName(), d.Aggression
		site.CrewCap, site.Stage = crewCap(s.campaign, loc, d, p), p.Stage
		if p.Faction == campaign.PlayerFaction {
			site.Skip = directorSkip(p) // the groups the player took over
		}
		dir.Update(site, s.tick)
		if dir.Ordered["raids"] > raids && (s.wm == nil || s.wm.currentID == loc.ID) {
			message.PostMessage("Alert", d.RaidAlert(loc))
		}
	}
}

// crewCap is a settlement presence's crew cap: its ship's tier's; for a
// planet-carried converting presence (the autonz) the capacity its
// conversion share allows; else 0 (none).
func crewCap(c *campaign.Campaign, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) int {
	if c == nil {
		return 0
	}
	if f := c.FleetByPresence(p.ID); f != nil {
		return d.CrewCap(f.ShipTier)
	}
	if d.Converts() && loc != nil {
		return d.ConvertCap(loc, p)
	}
	if isHiveDoctrine(d) && loc != nil {
		return d.CapacityAt(loc) // the hive grows to its site's capacity
	}
	return 0
}
