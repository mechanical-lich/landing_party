package generation

import (
	"fmt"
	"log"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// StationPrimer builds a station from a station plan (station_plan.md): a
// frame (spine, ring or cross) on every deck, the plan's modules allocated to
// decks by their band and run as structure scripts into slots that share
// walls with the frame's corridors, spare slots filled with ordinary rooms,
// two stair columns through every deck, and the dock with its arm on the
// dock's deck (deck 1 on a station of 4+ decks). The void around it is a
// margin. Without a plan in TerrainParams it rolls DefaultStationPlan.
type StationPrimer struct{}

func init() {
	RegisterPrimer("station", StationPrimer{})
	ModuleKinds["bay"] = ModuleKind{Script: "station_bay", Sizes: []string{"S", "M"}, Band: -1}
}

// Station frame constants.
const (
	depthSlack         = 10 // how far a docked wreck may reach into the margin
	stationDeckCost    = 12 // tiles one deck of difference counts as (origins, keep-outs)
	stationHazardHops  = 2  // a chapter's module keeps this many modules from the dock
	stationDefaultArm  = 10
	stationDefaultEdge = 14
	stationModulesPer  = 5 // real modules per deck before a band overflows
)

// Station region tags.
const (
	RegionStationDock = "station_dock"
	RegionStationHub  = "station_hub"
)

// ModuleRegion is the region tag at a module's centre ("module:lab").
func ModuleRegion(id string) string { return "module:" + id }

// slot is a rect hung off a corridor: its door is in the wall it shares with
// the corridor, at that wall's centre.
type slot struct {
	x, y, w, h   int
	doorX, doorY int
	// inX, inY step from the door into the slot.
	inX, inY int
}

// deckLayout is one deck's frame: corridor floor and wall cells, the slots
// hung off it, the stair column tiles and the dock.
type deckLayout struct {
	z            int
	floor, walls map[[2]int]bool
	slots        []slot
	dock         *slot
	arm          [][2]int // the arm's floor tiles (the dock's deck)
	airlocks     [][2]int
}

func newDeck(z int) *deckLayout {
	return &deckLayout{z: z, floor: map[[2]int]bool{}, walls: map[[2]int]bool{}}
}

// corridor lays a 3-wide corridor along a line (horizontal when y0 == y1,
// else vertical), its floor from one end point to the other, walls along
// both sides and across both ends.
func (d *deckLayout) corridor(x0, y0, x1, y1 int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	fx0, fx1, fy0, fy1 := x0, x1, y0-1, y1+1 // horizontal
	if y0 != y1 {
		fx0, fx1, fy0, fy1 = x0-1, x1+1, y0, y1
	}
	for y := fy0 - 1; y <= fy1+1; y++ {
		for x := fx0 - 1; x <= fx1+1; x++ {
			if x >= fx0 && x <= fx1 && y >= fy0 && y <= fy1 {
				d.floor[[2]int{x, y}] = true
			} else {
				d.walls[[2]int{x, y}] = true
			}
		}
	}
}

// box lays a walled room (floor inside, walls on its edge) as frame.
func (d *deckLayout) box(x, y, w, h int) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx == x || yy == y || xx == x+w-1 || yy == y+h-1 {
				d.walls[[2]int{xx, yy}] = true
			} else {
				d.floor[[2]int{xx, yy}] = true
			}
		}
	}
}

// sideRun hangs slots off one side of a straight corridor wall. horizontal
// says the wall runs along x at fixed wall coordinate (a row), else along y
// (a column); dir is -1 (slots on the low side: north or west) or +1. from
// and to bound the run along the wall; sizes are the slots' long x short
// sizes in order (a vertical run lays them transposed). It returns the slots
// and where the run ended.
func sideRun(horizontal bool, wall, dir, from, to int, sizes [][2]int, skip func(a, b int) (int, bool)) ([]slot, int) {
	var out []slot
	cur := from
	for _, sz := range sizes {
		along, depth := sz[0], sz[1]
		if skip != nil {
			if nc, jump := skip(cur, cur+along-1); jump {
				cur = nc
			}
		}
		if cur+along-1 > to {
			break
		}
		var s slot
		if horizontal {
			s = slot{x: cur, w: along, h: depth, doorX: cur + along/2, doorY: wall}
			if dir < 0 {
				s.y = wall - depth + 1
				s.inY = -1
			} else {
				s.y = wall
				s.inY = 1
			}
		} else {
			s = slot{y: cur, w: depth, h: along, doorX: wall, doorY: cur + along/2}
			if dir < 0 {
				s.x = wall - depth + 1
				s.inX = -1
			} else {
				s.x = wall
				s.inX = 1
			}
		}
		out = append(out, s)
		cur += along - 1 // neighbours share a wall
	}
	return out, cur
}

// run is one side of a corridor that slots hang off (see sideRun).
type run struct {
	horizontal bool
	wall, dir  int
	from, to   int
	skip       func(a, b int) (int, bool)
}

// layRuns hangs a deck's modules off its runs, each module on the run with
// the most length left, then fills every run to its end with bays.
func layRuns(dl *deckLayout, runs []run, mods []*placedModule, z int, rng *rand.Rand) {
	per := make([][]*placedModule, len(runs))
	sizes := make([][][2]int, len(runs))
	for _, m := range mods {
		best, bestLeft := -1, -1
		for i, r := range runs {
			try := append(append([][2]int(nil), sizes[i]...), m.size)
			slots, end := sideRun(r.horizontal, r.wall, r.dir, r.from, r.to, try, r.skip)
			if len(slots) == len(try) && r.to-end > bestLeft {
				best, bestLeft = i, r.to-end
			}
		}
		if best < 0 {
			continue // no room on this deck: the caller moves it
		}
		per[best] = append(per[best], m)
		sizes[best] = append(sizes[best], m.size)
	}
	for i, r := range runs {
		sz := sizes[i]
		for len(sz) < len(per[i])+40 {
			sz = append(sz, filler(rng))
		}
		slots, _ := sideRun(r.horizontal, r.wall, r.dir, r.from, r.to, sz, r.skip)
		placeOnSlots(dl, per[i], slots, z)
	}
}

// wreckSite is the room a frame keeps for a docked wreck: on deck z, south
// of a corridor whose south wall is row wall, a w x h hull at (x, y) with a
// one-row gangway gap above it.
type wreckSite struct {
	z, w, h int
	x, y    int
	wall    int
	ok      bool
	// decks are the decks to try, best first.
	decks []int
}

// wreckDecks are the decks a docked wreck may berth on, best first: the
// top one down, the dock's last, so it stands away from the landing.
func wreckDecks(decks, dockDeck int) []int {
	var out []int
	for z := decks - 1; z >= 0; z-- {
		if z != dockDeck {
			out = append(out, z)
		}
	}
	return append(out, dockDeck)
}

// placedModule is a module (or a filler bay) the primer is building.
type placedModule struct {
	id, kind string
	z        int
	size     [2]int
	plan     *PlanModule // nil for a filler
	slot     slot
	placed   bool
	// damage and damageKind are what the history left of it (ModuleDamage).
	damage     int
	damageKind string
}

// Prime lays the station. Params: margin (void round the station, default
// 14), dock_arm (the docking arm's length, default 10), station_plan.
func (StationPrimer) Prime(level *world.Level, params map[string]any, seed int64) error {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	level.AllocTerrain()
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				paintKind(level, x, y, z, world.TKSpace)
			}
		}
	}
	rng := rand.New(rand.NewSource(seed))
	plan := ParseStationPlan(params)
	if plan == nil {
		plan = DefaultStationPlan(seed, min(d, 5))
	}
	decks := clampInt(plan.Decks, 1, d)
	margin := paramInt(params, "margin", stationDefaultEdge)
	arm := paramInt(params, "dock_arm", stationDefaultArm)
	dockDeck := 0
	if decks >= 4 {
		dockDeck = 1
	}
	level.SurfaceZ = dockDeck
	level.AtmosphereZ = decks
	level.SpaceZ = decks

	mods := allocateModules(plan, decks, dockDeck, rng)
	frame := plan.Frame
	if frame == "" {
		frame = FrameSpine
	}
	layRng := rand.New(rand.NewSource(rng.Int63()))
	cx, cy := w/2, h/2
	// The station's body: inside the margin, less the dock and its arm on
	// the west.
	x0, x1 := margin, w-1-margin
	y0, y1 := margin, h-1-margin
	dockW := SlotSizes["M"][0]
	bodyX0 := x0 + arm + dockW - 1
	var wr *wreckSite
	if plan.Wreck != nil && plan.Wreck.W > 0 {
		wr = &wreckSite{w: plan.Wreck.W, h: plan.Wreck.H, decks: wreckDecks(decks, dockDeck)}
		wr.z = wr.decks[0]
	}
	var layouts []*deckLayout
	var columns [2][2]int // main and service stair columns: tile A of each
	// A module its deck has no room for moves, at its smallest size, to the
	// deck with the fewest modules it may use, and the frame is laid again.
	for attempt := 0; ; attempt++ {
		lr := rand.New(rand.NewSource(layRng.Int63()))
		for i := range mods {
			mods[i].placed = false
		}
		switch frame {
		case FrameRing:
			layouts, columns = ringFrame(mods, decks, dockDeck, bodyX0, x1, y0, y1, cx, cy, wr, lr)
		default:
			layouts, columns = spineFrame(mods, decks, dockDeck, frame == FrameCross, bodyX0, x1, y0, y1, cx, cy, wr, lr)
		}
		if wr != nil && !wr.ok && len(wr.decks) > 1 && attempt < 8 {
			// No berth on that deck: try the next.
			wr.decks = wr.decks[1:]
			wr.z = wr.decks[0]
			continue
		}
		if unplacedCount(mods) == 0 || attempt >= 12 {
			break
		}
		count := make([]int, decks)
		for _, m := range mods {
			count[m.z]++
		}
		for i := range mods {
			m := &mods[i]
			if m.placed || m.plan == nil {
				continue
			}
			m.size = SlotSizes[ModuleKinds[m.kind].Sizes[0]]
			if attempt == 0 {
				continue // first try the smaller size where it is
			}
			best := -1
			for z := 0; z < decks; z++ {
				if z == m.z || m.plan.Hazard && decks >= 3 && z == dockDeck {
					continue
				}
				if best < 0 || count[z] < count[best] {
					best = z
				}
			}
			if best >= 0 {
				count[m.z]--
				m.z = best
				count[best]++
			}
		}
	}
	// Paint the frame: walls first, then floors over them (junctions).
	for z, dl := range layouts {
		for p := range dl.walls {
			stampFrameCell(level, p[0], p[1], z, true)
		}
		for p := range dl.floor {
			stampFrameCell(level, p[0], p[1], z, false)
		}
		for _, p := range dl.arm {
			stampFrameCell(level, p[0], p[1], z, false)
		}
	}
	// Stairs: two columns of two tiles each; link z↔z+1 uses tile A on even
	// z and B on odd, so a deck has an up and a down at every column.
	for _, col := range columns {
		for z := 0; z < decks-1; z++ {
			tx := col[0] + z%2
			ty := col[1]
			for _, zz := range []int{z, z + 1} {
				level.SetFloor(tx, ty, zz, "hull_floor", world.TileVariantAt("hull_floor", tx, ty, zz))
				level.SetTerrainKind(tx, ty, zz, world.TKStructure)
			}
			level.SetMiddle(tx, ty, z, "stairs_up", 0)
			level.SetMiddle(tx, ty, z+1, "stairs_down", 0)
		}
	}
	// Spare slots become bays.
	for z, dl := range layouts {
		for i, sl := range dl.slots {
			mods = append(mods, placedModule{id: fmt.Sprintf("bay:%d:%d", z, i), kind: "bay", z: z, size: [2]int{sl.w, sl.h}, slot: sl, placed: true})
		}
	}
	// Decay: every module's damage once its deck is known.
	deckOf := map[string]int{}
	for _, m := range mods {
		if m.plan != nil {
			deckOf[m.id] = m.z
		}
	}
	dmg, dkind := plan.ModuleDamage(deckOf)
	deckLevel, deckKind := plan.DeckFloors(deckOf)
	for i := range mods {
		m := &mods[i]
		if m.plan != nil {
			m.damage, m.damageKind = dmg[m.id], dkind[m.id]
		} else {
			m.damage, m.damageKind = min(max(deckLevel[m.z], plan.StationDamage), 3), deckKind[m.z]
			if m.damageKind == "" {
				m.damageKind = plan.StationKind
			}
		}
	}
	decayRng := rand.New(rand.NewSource(seed ^ 0x446b79)) // "Dky"
	keyModule, logModule := keyAndLogModules(plan, mods)
	// Modules: each script into its slot, then its door.
	runner := structureRunner
	for i := range mods {
		m := &mods[i]
		if !m.placed {
			continue
		}
		s := m.slot
		for yy := s.y; yy < s.y+s.h; yy++ {
			for xx := s.x; xx < s.x+s.w; xx++ {
				stampStationCell(level, xx, yy, m.z, xx == s.x || yy == s.y || xx == s.x+s.w-1 || yy == s.y+s.h-1)
			}
		}
		p := moduleParams(m)
		if runner != nil {
			if err := runner(level, ModuleKinds[m.kind].Script, s.x, s.y, s.w, s.h, p, rng.Int63()); err != nil {
				log.Printf("station: module %s (%s): %v", m.id, ModuleKinds[m.kind].Script, err)
			}
		}
		decayModule(level, m, runner, decayRng)
		openDoor(level, s, m.z)
		ownFrameDoors(level, s, m.z)
		sealed := ""
		if m.plan != nil {
			sealed = m.plan.Sealed
		}
		switch {
		case sealed != "":
			// Sealed: a locked blast door the station's keycard opens.
			if d := spawnDoor(level, "blast_door", s.doorX, s.doorY, m.z, false); d != nil {
				dc := d.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
				dc.Locked, dc.KeyId = true, StationKeycard
			}
		case m.kind == "bridge":
			// The bridge stays shut until the power comes back.
			if d := spawnDoor(level, "airlock", s.doorX, s.doorY, m.z, true); d != nil {
				d.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked = true
				d.AddComponent(&components.PoweredComponent{Grid: "main", Unlock: true})
			}
		}
		if m.id == keyModule {
			spawnItem(level, StationKeycard, s.x+s.w/2, s.y+s.h/2-1, m.z)
		}
		if m.id == logModule {
			if e, err := factory.Create("datapad", s.x+s.w/2, s.y+s.h/2+1, m.z); err == nil {
				e.AddComponent(&components.DatapadComponent{Log: true})
				if e.HasComponent(rlcomponents.Description) {
					e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name = "Station Log"
				}
				level.AddEntity(e)
			}
		}
		name := level.RoomAt(s.x+s.w/2, s.y+s.h/2, m.z)
		mod := world.Module{ID: m.id, Kind: m.kind, X: s.x, Y: s.y, Z: m.z, W: s.w, H: s.h, Name: name,
			DoorX: s.doorX, DoorY: s.doorY, Sealed: sealed}
		if m.plan != nil {
			mod.Damage, mod.Event = m.damage, m.plan.Event
		}
		if m.plan != nil || m.kind != "bay" {
			level.Modules = append(level.Modules, mod)
			level.TagRegion(ModuleRegion(m.id), s.x+s.w/2, s.y+s.h/2, m.z)
		}
	}
	// A deck the nanites took is wired over in its corridors too.
	for z, dl := range layouts {
		if deckKind[z] == "nanite" && deckLevel[z] > 0 {
			for _, p := range sortedCells(dl.floor) {
				wireTile(level, p[0], p[1], z, deckLevel[z], decayRng)
			}
		}
	}
	if wr != nil && wr.ok {
		buildWreck(level, plan.Wreck, wr, runner, rng)
	} else if wr != nil {
		log.Printf("station: no berth for the docked wreck (%dx%d)", wr.w, wr.h)
	}
	// The dock's airlock pair: dock to arm, arm to space.
	for z, dl := range layouts {
		for _, p := range dl.airlocks {
			level.SetFloor(p[0], p[1], z, "hull_floor", world.TileVariantAt("hull_floor", p[0], p[1], z))
			level.ClearMiddle(p[0], p[1], z)
			spawnDoor(level, "airlock", p[0], p[1], z, true)
		}
	}
	// Anything with a floor is the station's structure.
	for z := 0; z < decks; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if t := level.GetTilePtr(x, y, z); t != nil && !t.Floor.IsEmpty() && !world.IsSpaceTile(t) {
					level.SetTerrainKind(x, y, z, world.TKStructure)
				}
			}
		}
	}
	// Region tags.
	for z := 0; z < decks; z++ {
		level.TagRegion(RegionStationHub, columns[0][0], columns[0][1]+1, z)
		level.TagRegion(fmt.Sprintf("station_deck:%d", z), columns[0][0], columns[0][1]+1, z)
	}
	if dock := level.ModuleByID("dock"); dock != nil {
		dx, dy, dz := dock.Center()
		level.TagRegion("starting_floor", dx, dy, dz)
		level.TagRegion(RegionStationDock, dx, dy, dz)
	}
	var big *world.Module
	for i := range level.Modules {
		m := &level.Modules[i]
		if m.Z == 0 && m.Kind != "dock" && (big == nil || m.W*m.H > big.W*big.H) {
			big = m
		}
	}
	if big != nil {
		level.TagRegion("station_room_large", big.X+big.W/2, big.Y+big.H/2, 0)
	}
	log.Printf("StationPrimer: %s %s station, %d decks, %d modules (%d unplaced)", plan.Purpose, frame, decks, len(level.Modules), unplacedCount(mods))
	return nil
}

func unplacedCount(mods []placedModule) int {
	n := 0
	for _, m := range mods {
		if !m.placed && m.plan != nil {
			n++
		}
	}
	return n
}

// moduleParams are the params a module's script gets: the frame's (z,
// frame, occupants none, damage) and the plan's own.
func moduleParams(m *placedModule) map[string]any {
	p := map[string]any{"z": float64(m.z), "frame": "station", "occupants": "none"}
	if m.damage > 0 {
		p["damage"] = float64(m.damage)
	}
	if m.damageKind != "" {
		p["damage_kind"] = m.damageKind
	}
	if m.plan == nil {
		return p
	}
	for k, v := range m.plan.Params {
		p[k] = v
	}
	if m.plan.Occupants != "" {
		p["occupants"] = m.plan.Occupants
	}
	if m.plan.Sealed != "" {
		p["sealed"] = m.plan.Sealed
	}
	return p
}

// stampFrameCell paints a frame cell: hull floor, and a hull wall unless an
// earlier floor pass already opened it.
func stampFrameCell(level *world.Level, x, y, z int, wall bool) {
	if x < 0 || y < 0 || x >= level.GetWidth() || y >= level.GetHeight() {
		return
	}
	level.SetFloor(x, y, z, "hull_floor", world.TileVariantAt("hull_floor", x, y, z))
	if wall {
		level.SetMiddle(x, y, z, "hull_wall", world.TileVariantAt("hull_wall", x, y, z))
	} else {
		level.ClearMiddle(x, y, z)
	}
	level.SetTerrainKind(x, y, z, world.TKStructure)
}

// openDoor opens a slot's door and clears the way through it: the door
// tile, the tile inside and the corridor tile outside lose any solid middle
// and any solid furniture (not a door) a script put there.
func openDoor(level *world.Level, s slot, z int) {
	for _, p := range [][2]int{{s.doorX, s.doorY}, {s.doorX + s.inX, s.doorY + s.inY}, {s.doorX - s.inX, s.doorY - s.inY}} {
		level.SetFloor(p[0], p[1], z, "hull_floor", world.TileVariantAt("hull_floor", p[0], p[1], z))
		if t := level.GetTilePtr(p[0], p[1], z); t != nil && !t.Middle.IsEmpty() && (t.IsSolid() || world.IsSpaceTile(t)) {
			level.ClearMiddle(p[0], p[1], z)
		}
		removeSolidAt(level, p[0], p[1], z)
	}
}

// removeSolidAt removes solid furniture (not doors) covering (x, y, z).
func removeSolidAt(level *world.Level, x, y, z int) {
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range append([]*ecs.Entity(nil), list...) {
			if e == nil || !e.HasComponent(rlcomponents.Solid) || e.HasComponent(rlcomponents.Door) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetZ() != z {
				continue
			}
			ew, eh := 1, 1
			if e.HasComponent(rlcomponents.Size) {
				sc := e.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
				ew, eh = max(sc.Width, 1), max(sc.Height, 1)
			}
			ex, ey := pc.GetX()-ew/2, pc.GetY()-eh/2
			if x >= ex && x < ex+ew && y >= ey && y < ey+eh {
				level.RemoveEntity(e)
			}
		}
	}
}

// ownFrameDoors gives the colony every door a module's script put in the
// wall it shares with the frame (a hangar's bay doors facing the corridor),
// so the way in is never shut to its colonists.
func ownFrameDoors(level *world.Level, s slot, z int) {
	for _, e := range level.Entities {
		if e == nil || !e.HasComponent(rlcomponents.Door) || !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		x, y := pc.GetX(), pc.GetY()
		onWall := s.inY != 0 && y == s.doorY && x >= s.x && x < s.x+s.w || s.inX != 0 && x == s.doorX && y >= s.y && y < s.y+s.h
		if pc.GetZ() == z && onWall {
			e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).OwnedBy = "colony"
		}
	}
}

// spawnDoor puts a door blueprint on (x, y, z); owned doors open for the
// colony (an unowned door blocks everyone's paths).
func spawnDoor(level *world.Level, bp string, x, y, z int, owned bool) *ecs.Entity {
	removeSolidAt(level, x, y, z)
	e, err := factory.Create(bp, x, y, z)
	if err != nil || !e.HasComponent(rlcomponents.Door) {
		return nil
	}
	if owned {
		e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).OwnedBy = "colony"
	}
	level.AddEntity(e)
	return e
}

// StationKeycard is the item that opens a station's sealed doors.
const StationKeycard = "station_keycard"

// spawnItem drops an item blueprint on (x, y, z).
func spawnItem(level *world.Level, bp string, x, y, z int) {
	if e, err := factory.Create(bp, x, y, z); err == nil {
		level.AddEntity(e)
	}
}

// keyAndLogModules pick where a station keeps its keycard (when anything is
// sealed: the bridge, else the generator) and its log console (the plan's
// log module, else the archive, the relay, the bridge, the generator).
func keyAndLogModules(plan *StationPlan, mods []placedModule) (key, logm string) {
	placed := map[string]bool{}
	sealed := false
	for _, m := range mods {
		if m.placed && m.plan != nil {
			placed[m.id] = true
			if m.plan.Sealed != "" {
				sealed = true
			}
		}
	}
	first := func(ids ...string) string {
		for _, id := range ids {
			if placed[id] {
				return id
			}
		}
		return ""
	}
	if sealed {
		key = first("bridge", "generator")
	}
	for _, m := range plan.Modules {
		if m.Log && placed[m.ID] {
			logm = m.ID
		}
	}
	if logm == "" {
		logm = first("archive", "relay", "bridge", "generator")
	}
	return key, logm
}

// allocateModules gives every plan module a deck: its band's preferred decks
// first, a chapter's or creature-spawning module never on the dock's deck
// (on 3+ decks), at most stationModulesPer real modules a deck before a band
// spills over. Deterministic for the seed.
func allocateModules(plan *StationPlan, decks, dockDeck int, rng *rand.Rand) []placedModule {
	count := make([]int, decks)
	var out []placedModule
	prefs := func(band int) []int {
		var p []int
		add := func(z int) {
			if z >= 0 && z < decks {
				for _, q := range p {
					if q == z {
						return
					}
				}
				p = append(p, z)
			}
		}
		top := decks - 1
		switch band {
		case BandDock:
			add(dockDeck)
		case BandBottom:
			add(0)
		case BandLow:
			add(dockDeck)
			add(dockDeck + 1)
		case BandMiddle:
			for z := dockDeck + 1; z <= top-1; z++ {
				add(z)
			}
		case BandUpper:
			add(top - 1)
			add(top - 2)
		case BandTop:
			add(top)
		}
		// Then the rest, nearest the first choice first.
		first := dockDeck
		if len(p) > 0 {
			first = p[0]
		}
		for dz := 1; dz < decks; dz++ {
			add(first + dz)
			add(first - dz)
		}
		return p
	}
	order := make([]int, len(plan.Modules))
	for i := range order {
		order[i] = i
	}
	// The dock first, then hazards (they have the fewest decks to choose
	// from), then the rest in plan order.
	sort.SliceStable(order, func(a, b int) bool {
		ma, mb := plan.Modules[order[a]], plan.Modules[order[b]]
		ra, rb := 2, 2
		if ma.Kind == "dock" {
			ra = 0
		} else if ma.Hazard {
			ra = 1
		}
		if mb.Kind == "dock" {
			rb = 0
		} else if mb.Hazard {
			rb = 1
		}
		return ra < rb
	})
	for _, i := range order {
		m := &plan.Modules[i]
		mk, ok := ModuleKinds[m.Kind]
		if !ok {
			continue
		}
		cands := prefs(mk.Band)
		if m.Hazard && decks >= 3 {
			kept := cands[:0]
			for _, z := range cands {
				if z != dockDeck {
					kept = append(kept, z)
				}
			}
			cands = kept
		}
		z := cands[0]
		for _, c := range cands {
			if count[c] < stationModulesPer || m.Kind == "dock" {
				z = c
				break
			}
		}
		count[z]++
		size := SlotSizes[mk.Sizes[rng.Intn(len(mk.Sizes))]]
		out = append(out, placedModule{id: m.ID, kind: m.Kind, z: z, size: size, plan: m})
	}
	return out
}

// deckMods lists the modules allocated to deck z, the dock left out.
func deckMods(mods []placedModule, z int) []*placedModule {
	var out []*placedModule
	for i := range mods {
		if mods[i].z == z && mods[i].kind != "dock" {
			out = append(out, &mods[i])
		}
	}
	return out
}

// filler is a spare slot's room.
func filler(rng *rand.Rand) [2]int {
	if rng.Intn(3) == 0 {
		return SlotSizes["S"]
	}
	return SlotSizes["M"]
}

// spineFrame lays a spine (and, for a cross, a perpendicular corridor through
// its middle) on every deck: a 3-wide corridor along x at cy, modules
// alternating north and south of it, spare length filled with bays; the
// dock at the west end of the dock's deck with its arm running west into
// the margin. The top and bottom decks of a 4+ deck station are shorter.
func spineFrame(mods []placedModule, decks, dockDeck int, cross bool, bodyX0, x1, y0, y1, cx, cy int, wr *wreckSite, rng *rand.Rand) ([]*deckLayout, [2][2]int) {
	layouts := make([]*deckLayout, decks)
	maxDepth := SlotSizes["XL"][1]
	// The corridor's natural length per deck: the longer of its two sides.
	need := 0
	for z := 0; z < decks; z++ {
		n, s := 0, 0
		for i, m := range deckMods(mods, z) {
			if i%2 == 0 {
				n += m.size[0] - 1
			} else {
				s += m.size[0] - 1
			}
		}
		need = max(need, max(n, s)+2)
	}
	full := x1 - bodyX0 + 1
	length := clampInt(max(need, full*3/4), min(40, full), full)
	start := bodyX0
	if length < full {
		start = bodyX0 + (full-length)/2
	}
	short := length * 3 / 4
	mainX := start + length/2
	serviceX := start + length/2 + short/2 - 4
	if cross {
		serviceX = start + length/2 - short/2 + 3
	}
	for z := 0; z < decks; z++ {
		dl := newDeck(z)
		layouts[z] = dl
		ls, le := start, start+length-1
		if decks >= 4 && (z == 0 || z == decks-1) {
			ls, le = mainX-short/2, mainX+short/2
		}
		if z == dockDeck {
			ls = start
		}
		dl.corridor(ls, cy, le, cy)
		var band func(a, b int) (int, bool)
		runs := spineRuns(ls, le, cy, nil)
		armed := false
		// The wreck's deck keeps its berth clear of the cross's arm, or lays
		// no arm.
		armFits := true
		if wr != nil && wr.z == z {
			reach := 2 + maxDepth + 1
			armFits = le+1-wr.w+1 > mainX+reach || ls+wr.w < mainX-reach
		}
		if cross && armFits && cy-y0 >= maxDepth+2+SlotSizes["M"][0] {
			armed = true
			// The cross's vertical corridor through the middle, slots west and
			// east of it beyond the spine's slots.
			vy0, vy1 := cy-2-maxDepth-SlotSizes["L"][0]+1, cy+2+maxDepth+SlotSizes["L"][0]-1
			vy0, vy1 = max(vy0, y0+2), min(vy1, y1-2)
			dl.corridor(mainX, vy0, mainX, vy1)
			band = func(a, b int) (int, bool) {
				if b >= mainX-2 && a <= mainX+2 {
					return mainX + 2, true
				}
				return a, false
			}
			runs = spineRuns(ls, le, cy, band)
			for _, dir := range []int{-1, 1} {
				runs = append(runs,
					run{wall: mainX + 2*dir, dir: dir, from: vy0 - 1, to: cy - 2 - maxDepth},
					run{wall: mainX + 2*dir, dir: dir, from: cy + 2 + maxDepth, to: vy1 + 1})
			}
		}
		if wr != nil && wr.z == z && le-ls > wr.w+4 && cy+4+wr.h < y1+depthSlack {
			// The far end of the south side is the wreck's berth; on a cross,
			// clear of the vertical arm and its rooms (east end, else west).
			reach := 2 + maxDepth + 1
			east, west := le+1-wr.w+1, ls+1
			switch {
			case !armed || east > mainX+reach:
				wr.x, wr.y, wr.wall, wr.ok = east, cy+4, cy+2, true
				runs[1].to = wr.x - 2
			case west+wr.w-1 < mainX-reach:
				wr.x, wr.y, wr.wall, wr.ok = west, cy+4, cy+2, true
				runs[1].from = wr.x + wr.w + 1
			}
		}
		layRuns(dl, runs, deckMods(mods, z), z, rng)
		if z == dockDeck {
			dockAt(dl, mods, ls-1, cy, z)
		}
	}
	return layouts, [2][2]int{{mainX - 1, cy - 1}, {serviceX, cy - 1}}
}

// spineRuns are a spine's two sides, north and south of its corridor.
func spineRuns(ls, le, cy int, band func(a, b int) (int, bool)) []run {
	return []run{
		{horizontal: true, wall: cy - 2, dir: -1, from: ls - 1, to: le + 1, skip: band},
		{horizontal: true, wall: cy + 2, dir: 1, from: ls - 1, to: le + 1, skip: band},
	}
}

// placeOnSlots puts modules on the first slots and keeps the rest as bays.
func placeOnSlots(dl *deckLayout, mods []*placedModule, slots []slot, z int) {
	for i, s := range slots {
		if i < len(mods) {
			mods[i].slot, mods[i].placed = s, true
			continue
		}
		dl.slots = append(dl.slots, s)
	}
}

// dockAt puts the dock against a west wall at column doorX (its door at
// row cy), and its arm running west from it.
func dockAt(dl *deckLayout, mods []placedModule, doorX, cy, z int) {
	for i := range mods {
		m := &mods[i]
		if m.kind != "dock" {
			continue
		}
		dw, dh := SlotSizes["M"][0], SlotSizes["M"][1]
		s := slot{x: doorX - dw + 1, y: cy - dh/2, w: dw, h: dh, doorX: doorX, doorY: cy, inX: -1}
		m.slot, m.placed, m.z = s, true, z
		dl.dock = &s
		// The arm: a walled 3-wide tube from the dock's west wall.
		ax := s.x
		for x := ax - stationDefaultArm; x < ax; x++ {
			for y := cy - 2; y <= cy+2; y++ {
				if y == cy-2 || y == cy+2 || x == ax-stationDefaultArm {
					dl.walls[[2]int{x, y}] = true
				} else {
					dl.arm = append(dl.arm, [2]int{x, y})
				}
			}
		}
		dl.airlocks = append(dl.airlocks, [2]int{ax, cy}, [2]int{ax - stationDefaultArm, cy})
		return
	}
}

// ringFrame lays a ring on every deck: a square loop corridor round a hub,
// joined to it by spokes (east and west always, north and south by roll),
// modules outside the loop on its four sides, spare sides filled with bays;
// the dock on the west side of the dock's deck at the west spoke's row, its
// arm running west. The top and bottom decks of a 4+ deck station have a
// smaller loop.
func ringFrame(mods []placedModule, decks, dockDeck, bodyX0, x1, y0, y1, cx, cy int, wr *wreckSite, rng *rand.Rand) ([]*deckLayout, [2][2]int) {
	layouts := make([]*deckLayout, decks)
	depth := SlotSizes["XL"][1]
	const hub = 4 // half-size: a 9x9 hub
	// The full loop's outer box: slots up to depth deep outside it, the dock
	// and its arm west of it.
	fx0, fx1 := bodyX0, x1-depth+1
	fy0, fy1 := y0+depth-1, y1-depth+1
	hcx, hcy := (fx0+fx1)/2, (fy0+fy1)/2
	for z := 0; z < decks; z++ {
		dl := newDeck(z)
		layouts[z] = dl
		inset := 0
		if decks >= 4 && (z == 0 || z == decks-1) {
			inset = 6
		}
		rx0, rx1, ry0, ry1 := fx0+inset, fx1-inset, fy0+inset, fy1-inset
		const least = 2*hub + 13 // room for the hub and the loop round it
		if rx1-rx0 < least {
			rx0, rx1 = hcx-least/2, hcx+least/2
		}
		if ry1-ry0 < least {
			ry0, ry1 = hcy-least/2, hcy+least/2
		}
		// The loop: four corridors on the centre lines of its sides, so its
		// outer walls are the box's edges.
		dl.corridor(rx0+2, ry0+2, rx1-2, ry0+2)
		dl.corridor(rx0+2, ry1-2, rx1-2, ry1-2)
		dl.corridor(rx0+2, ry0+2, rx0+2, ry1-2)
		dl.corridor(rx1-2, ry0+2, rx1-2, ry1-2)
		dl.box(hcx-hub, hcy-hub, 2*hub+1, 2*hub+1)
		// Spokes.
		dl.corridor(rx0+4, hcy, hcx-hub, hcy)
		dl.corridor(hcx+hub, hcy, rx1-4, hcy)
		if rng.Intn(2) == 0 || z == dockDeck {
			dl.corridor(hcx, ry0+4, hcx, hcy-hub)
		}
		if rng.Intn(2) == 0 {
			dl.corridor(hcx, hcy+hub, hcx, ry1-4)
		}
		// Modules round the outside; the west side leaves the dock's rows
		// free on its deck.
		var westSkip func(a, b int) (int, bool)
		if z == dockDeck {
			dh := SlotSizes["M"][1]
			westSkip = func(a, b int) (int, bool) {
				if b >= hcy-dh/2 && a <= hcy+dh/2 {
					return hcy + dh/2, true
				}
				return a, false
			}
		}
		southTo := rx1
		if wr != nil && wr.z == z && rx1-rx0 > wr.w+2 && ry1+2+wr.h < y1+depthSlack {
			// The east end of the south side is the wreck's berth.
			wr.x, wr.y, wr.wall, wr.ok = rx1-wr.w+1, ry1+2, ry1, true
			southTo = wr.x - 2
		}
		layRuns(dl, []run{
			{horizontal: true, wall: ry0, dir: -1, from: rx0, to: rx1},
			{horizontal: true, wall: ry1, dir: 1, from: rx0, to: southTo},
			{wall: rx0, dir: -1, from: ry0, to: ry1, skip: westSkip},
			{wall: rx1, dir: 1, from: ry0, to: ry1},
		}, deckMods(mods, z), z, rng)
		if z == dockDeck {
			dockAt(dl, mods, rx0, hcy, z)
		}
	}
	// Main column in the hub, service column in the east spoke.
	return layouts, [2][2]int{{hcx - 1, hcy - 2}, {hcx + hub + 1, hcy + 1}}
}

// buildWreck docks the plan's wreck in its berth: crashed_ship on the
// berth's deck (frame=station: no furrow), then a gangway from the
// corridor's wall to the hull's, opened where the tile inside the hull is
// clear. The hull becomes the level's "wreck" module.
func buildWreck(level *world.Level, pw *PlanWreck, wr *wreckSite, runner StructureRunnerFunc, rng *rand.Rand) {
	p := map[string]any{}
	for k, v := range pw.Params {
		p[k] = v
	}
	p["z"], p["frame"] = float64(wr.z), "station"
	if runner != nil {
		if err := runner(level, "crashed_ship", wr.x, wr.y, wr.w, wr.h, p, rng.Int63()); err != nil {
			log.Printf("station: docked wreck: %v", err)
		}
	}
	z := wr.z
	walkable := func(x, y int) bool {
		t := level.GetTilePtr(x, y, z)
		return t != nil && !t.Floor.IsEmpty() && !t.IsSolid() && !world.IsSpaceTile(t)
	}
	cx := wr.x + wr.w/2
	for d := 0; d < wr.w/2-2; d++ {
		for _, gx := range []int{cx + d, cx - d} {
			if !walkable(gx, wr.wall-1) || level.GetSolidEntityAt(gx, wr.wall-1, z) != nil {
				continue
			}
			// The hull's top wall in this column (a broken hull sits a row or
			// two low), and the deck inside it.
			top := -1
			for y := wr.y; y < wr.y+4 && y < wr.y+wr.h-1; y++ {
				if t := level.GetTilePtr(gx, y, z); t != nil && !t.Floor.IsEmpty() {
					top = y
					break
				}
			}
			if top < 0 {
				continue
			}
			in := top + 1
			if t := level.GetTilePtr(gx, in, z); t == nil || t.Floor.IsEmpty() || t.IsSolid() && !t.Middle.IsEmpty() && world.TileDefinitions[t.Middle.Type].Name == "hull_wall" {
				continue // space, or a section's wall
			}
			for y := wr.wall; y <= top; y++ {
				stampFrameCell(level, gx, y, z, false)
				removeSolidAt(level, gx, y, z)
				if y > wr.wall && y < top {
					stampFrameCell(level, gx-1, y, z, true)
					stampFrameCell(level, gx+1, y, z, true)
				}
			}
			if t := level.GetTilePtr(gx, in, z); t.IsSolid() {
				level.ClearMiddle(gx, in, z)
			}
			removeSolidAt(level, gx, in, z)
			spawnDoor(level, "airlock", gx, wr.wall+1, z, true)
			level.Modules = append(level.Modules, world.Module{ID: WreckModule, Kind: "wreck", X: wr.x, Y: wr.y, Z: z, W: wr.w, H: wr.h,
				Name: level.RoomAt(wr.x+wr.w/2, wr.y+wr.h/2, z), DoorX: gx, DoorY: top, Event: pw.Event})
			level.TagRegion(ModuleRegion(WreckModule), wr.x+wr.w/2, wr.y+wr.h/2, z)
			return
		}
	}
	log.Printf("station: the docked wreck has no clear way in")
}

// sortedCells lists a cell set in a fixed order (decay draws from a seeded
// stream).
func sortedCells(m map[[2]int]bool) [][2]int {
	out := make([][2]int, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// wireTile lays nanite wire over a bare deck tile, more of them the higher
// the damage (1: 15%, 2: 40%, 3: 60%).
func wireTile(level *world.Level, x, y, z, damage int, rng *rand.Rand) {
	t := level.GetTilePtr(x, y, z)
	if t == nil || t.Floor.IsEmpty() || !t.Middle.IsEmpty() || world.TileDefinitions[t.Floor.Type].Name != "hull_floor" {
		return
	}
	if rng.Intn(100) < []int{0, 15, 40, 60}[min(damage, 3)] {
		level.SetFloor(x, y, z, "blue_wires", world.TileVariantAt("blue_wires", x, y, z))
	}
}

// decayModule gives a built module its damage's look (station_plan.md,
// section 8; the script already wore its walls by damage): a breach blows
// it open to space (hull_breach into its rect at damage 3), nanites wire its
// floors, a meltdown leaves rubble in the corridor outside its door, an
// abandoned station's lockers keep a third of what they held and its doors
// stand open.
func decayModule(level *world.Level, m *placedModule, runner StructureRunnerFunc, rng *rand.Rand) {
	s := m.slot
	switch m.damageKind {
	case "breach":
		if m.damage >= 3 && runner != nil {
			p := moduleParams(m)
			p["room_hall"] = "Breached " + level.RoomAt(s.x+s.w/2, s.y+s.h/2, m.z)
			if err := runner(level, "hull_breach", s.x, s.y, s.w, s.h, p, rng.Int63()); err != nil {
				log.Printf("station: breach %s: %v", m.id, err)
			}
		}
	case "nanite":
		for y := s.y + 1; y < s.y+s.h-1; y++ {
			for x := s.x + 1; x < s.x+s.w-1; x++ {
				wireTile(level, x, y, m.z, m.damage, rng)
			}
		}
	case "meltdown":
		// Rubble on the corridor's far lanes either side of the door, never
		// in front of it, so the way stays open.
		for d := 2; d <= 5; d++ {
			for _, sign := range []int{-1, 1} {
				x, y := s.doorX-s.inX*3, s.doorY-s.inY*3
				if s.inX == 0 {
					x += sign * d
				} else {
					y += sign * d
				}
				if t := level.GetTilePtr(x, y, m.z); t != nil && !t.Floor.IsEmpty() && t.Middle.IsEmpty() && rng.Intn(2) == 0 {
					level.SetMiddle(x, y, m.z, "rubble_pile", 0)
				}
			}
		}
	case "abandoned":
		for _, e := range append([]*ecs.Entity(nil), level.Entities...) {
			if e == nil || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetZ() != m.z || pc.GetX() < s.x || pc.GetX() >= s.x+s.w || pc.GetY() < s.y || pc.GetY() >= s.y+s.h {
				continue
			}
			if e.HasComponent(components.Storage) {
				st := e.GetComponent(components.Storage).(*components.StorageComponent)
				kept := st.Items[:0]
				for _, it := range st.Items {
					if rng.Intn(3) == 0 {
						kept = append(kept, it)
					}
				}
				st.Items = kept
			}
			if e.HasComponent(rlcomponents.Door) {
				d := e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
				if !d.Locked {
					d.Open = true
					e.RemoveComponent(rlcomponents.Solid)
				}
			}
		}
	}
}

// stampStationCell paints a station floor cell with optional wall middle.
// Floor is always hull_floor (so destroyed walls leave a walkable cell).
// If isWall, Middle gets hull_wall, unless this cell was already cleared
// by a previous interior pass (lets corridors and doorways punch through).
func stampStationCell(level *world.Level, x, y, z int, isWall bool) {
	preserveInterior := level.GetTerrainKind(x, y, z) == world.TKStructure && interiorMiddle(level, x, y, z)
	level.SetFloor(x, y, z, "hull_floor", world.TileVariantAt("hull_floor", x, y, z))
	if isWall && !preserveInterior {
		level.SetMiddle(x, y, z, "hull_wall", world.TileVariantAt("hull_wall", x, y, z))
	} else if !isWall {
		level.ClearMiddle(x, y, z)
	}
	level.SetTerrainKind(x, y, z, world.TKStructure)
}

func interiorMiddle(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	return t != nil && t.Middle.IsEmpty()
}
