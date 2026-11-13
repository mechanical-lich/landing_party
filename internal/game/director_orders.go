package game

import (
	"hash/fnv"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/director"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// Observer mode (phase C1; docs/developer/factions.md, "Observer mode"): the
// director runs the player's settlement on every planet, the cursor gives no
// orders, incidents are answered Ignore and the Star Map clock never stops.
// It is the config flag observerMode (data/config.local.json; a vmtest
// script's header config).

// observerMode reports whether the ground plays itself: an observer run, or
// the config flag (on a normal campaign that is the lock alone: the
// settlement director runs the colony, the cursor gives no orders, the
// marker shows; the ship and its popups stay the player's).
func observerMode() bool {
	if observerRunActive {
		return true
	}
	if observerLock != nil {
		return *observerLock
	}
	return config.Global().ObserverMode
}

// observerLock is the console's observer on|off for the loaded campaign,
// nil when the console has not touched it (the config flag decides). It is
// reset by NewWorldManager, and the config flag is never written, so the
// lock cannot leak into the next New Expedition (which reads the flag as
// "start an observer run").
var observerLock *bool

// observerRunActive is the loaded campaign's Observer (set by
// NewWorldManager): no player ship, the expedition a fleet (observer.go).
var observerRunActive bool

// observerRun reports whether the loaded campaign is an observer run.
func observerRun() bool { return observerRunActive }

// setObserverMode turns the observer lock on or off for the loaded campaign
// (the console's observer on|off on a normal campaign); the config flag is
// left alone, and an observer run stays one.
func setObserverMode(on bool) { observerLock = &on }

// directorOrders are the player's own order paths, for the director: the
// build and mine cursors' queue functions, a click's haul, the research and
// crafting modals' requests and the Cancel Task order.
type directorOrders struct{ s *MainState }

func (o directorOrders) Build(kind string, x, y, z int) bool { return o.s.queueBuild(kind, x, y, z) }
func (o directorOrders) Mine(x, y, z int) bool               { return o.s.queueMine(x, y, z) }
func (o directorOrders) Dig(x, y, z int) bool                { return o.s.queueDig(x, y, z) }
func (o directorOrders) Convert(x, y, z int, tile string) bool {
	return o.s.queueConvert(x, y, z, tile)
}

// queueConvert is the player's ConvertAction order: refused until the
// settlement knows ConvertTech (no research entry gives it yet).
func (s *MainState) queueConvert(x, y, z int, tile string) bool {
	if !s.hasTech(ConvertTech) {
		return false
	}
	return orderConvert(s.MainSettlement, s.level, x, y, z, tile)
}

func (o directorOrders) Retrieve(item *ecs.Entity) bool {
	if o.s.MainSettlement == nil || !item.HasComponent(rlcomponents.Position) {
		return false
	}
	pc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	o.s.queueRetrieve(item, pc.GetX(), pc.GetY(), pc.GetZ())
	return true
}

func (o directorOrders) Research(tech string, station *ecs.Entity) bool {
	return o.s.addResearchTask(tech, station)
}

func (o directorOrders) Craft(recipe string, station *ecs.Entity) bool {
	return o.s.addCraftTask(recipe, station)
}

func (o directorOrders) Cancel(t *task.Task) { cancelOrder(o.s.MainSettlement, t) }

// directorSite is the director's view of this level's settlement.
func (s *MainState) directorSite() *director.Site {
	if s.MainSettlement == nil || s.level == nil {
		return nil
	}
	st := s.MainSettlement
	return &director.Site{
		Level: s.level, Colony: s.colonyName(),
		CX: st.CenterX, CY: st.CenterY, CZ: st.CenterZ,
		Tasks: &st.Tasks, Known: s.knownTechs(),
		Doctrine: s.doctrineSettlement(), Orders: directorOrders{s},
	}
}

// directorSeed is the settlement's seed: the campaign's and the location's.
func (s *MainState) directorSeed() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s.locationID))
	seed := int64(h.Sum64() >> 1)
	if s.campaign != nil {
		seed ^= s.campaign.Seed * 1000003
	}
	return seed
}

// stepDirector runs the director for this level's settlement once a tick in
// observer mode (or when a test attached one), never on The Ship.
func (s *MainState) stepDirector() {
	if s.director == nil {
		if !observerMode() || s.locationID == "" && s.campaign != nil {
			return
		}
		s.director = director.New(s.directorSeed())
	}
	if site := s.directorSite(); site != nil {
		s.director.Update(site, s.tick)
	}
}

func (o directorOrders) Attack(target *ecs.Entity) bool {
	return orderAttack(o.s.MainSettlement, target)
}
func (o directorOrders) Equip(blueprint string, _ *ecs.Entity) bool {
	return orderEquip(o.s.MainSettlement, o.s.level, blueprint)
}
func (o directorOrders) EquipWorker(worker *ecs.Entity, blueprint string) *task.Task {
	return orderEquipWorker(worker, blueprint)
}
func (o directorOrders) Send(worker, target *ecs.Entity) *task.Task {
	return orderSend(worker, target)
}
func (o directorOrders) Move(worker *ecs.Entity, x, y, z int) *task.Task {
	return orderMove(worker, x, y, z)
}
func (o directorOrders) Deploy(worker *ecs.Entity, x, y, z int, as string, floor []string, ticks int) *task.Task {
	return orderDeploy(o.s.level, worker, x, y, z, as, floor, ticks)
}
func (o directorOrders) Hold(worker *ecs.Entity, x, y, z int) *task.Task {
	return settlementOrders{}.Hold(worker, x, y, z)
}

// seedSimRand seeds the global math/rand source the simulation draws from
// (sleep rolls, script rnd_int and find_open_tile, GameMaster spawns,
// deposit richness, mlge and ml-rogue-lib dice and combat) from the
// campaign's seed and day, so the same campaign plays the same way from the
// same point: a soak run twice, a save loaded twice. go.mod's godebug
// randseednop=0 makes rand.Seed take effect. nil (a skirmish) leaves it.
func seedSimRand(c *campaign.Campaign) {
	if c == nil {
		return
	}
	rand.Seed(c.Seed*1_000_003 + int64(c.StarDate)*7_919 + 1) //nolint:staticcheck // deliberate: the sim's global source
}
