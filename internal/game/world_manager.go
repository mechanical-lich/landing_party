package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/ship"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/state"
)

// installCampaignStorageHook directs worker crafting/building to draw only
// from the live level's storage. Ship hold is managed separately via explicit
// beam operations in the Star Map.
func installCampaignStorageHook(c *campaign.Campaign) {
	workerai.StorageProviderFor = func(level *world.Level, settlementName string) (storage.Provider, []string) {
		return storage.LevelProvider{Level: level}, []string{settlementName}
	}
}

const campaignDir = "saves/campaign"

func campaignRoot(name string) string {
	return filepath.Join(campaignDir, sanitizeName(name))
}

// WorldManager owns the campaign and performs the live-level swap: freezing the
// current location to disk and activating (resuming or generating) another.
type WorldManager struct {
	Campaign *campaign.Campaign
	// loaded holds every location kept in memory, by location ID: the one in orbit
	// plus staffed ones left behind. All are "parked" (level intact, global event
	// listeners detached) until Resume re-attaches the one being entered.
	loaded map[string]*MainState
	// currentID is the loaded location in orbit, "" if none. See Current.
	currentID string
	// lastVisit orders loaded locations for eviction: higher is more recent.
	lastVisit map[string]int
	visits    int

	// owed is each away planet's background ticks due but not yet run, and
	// lastServed the planet the round-robin paid last. See TickBackground.
	owed       map[string]int
	lastServed string

	// lastDamage is the tick of the last colonist damage on each background level,
	// for grouping hits into one incident. incidents queues those awaiting an answer
	// and popup shows them. See incident.go.
	lastDamage    map[string]int
	incidents     []Incident
	notifications []Notification
	popup         *incidentPopup

	// pauseReason is who set paused, for the HUD marker: "star map", "incident".
	pauseReason string

	// shipLevel is The Ship — the always-loaded home base (a parked MainState).
	// Unlike locations it is never frozen to disk while the campaign is active,
	// and it ticks in the background. See docs/developer/the_ship.md.
	shipLevel *MainState

	// paused stops all background ticking (the Star-Map pause toggle).
	paused bool

	// Cached landing zone for the loaded location so every beam-down arrives
	// at the same plaza (recomputed on Travel).
	landSet             bool
	landX, landY, landZ int
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// landingZone returns the stable beam-down anchor for the loaded level: the
// colony settlement centre if known, else a one-time plaza search (cached).
func (wm *WorldManager) landingZone() (int, int, int) {
	if wm.landSet {
		return wm.landX, wm.landY, wm.landZ
	}
	ms := wm.Current()
	if ms != nil && ms.MainSettlement != nil {
		wm.landX, wm.landY, wm.landZ = ms.MainSettlement.CenterX, ms.MainSettlement.CenterY, ms.MainSettlement.CenterZ
	} else if ms != nil && ms.level != nil {
		x, y, z := findStartingPlaza(ms.level, ms.level.SurfaceZ, 5)
		if x == -1 {
			x, y, z = ms.camera.X, ms.camera.Y, ms.camera.Z
		}
		wm.landX, wm.landY, wm.landZ = x, y, z
	}
	wm.landSet = true
	return wm.landX, wm.landY, wm.landZ
}

// freeLandingTile spirals out from the landing anchor to the first standable,
// unoccupied tile so beamed colonists land together but never stack.
func freeLandingTile(level *world.Level, cx, cy, cz int) (int, int, int) {
	standableFree := func(x, y, z int) bool {
		return isStandable(level, x, y, z) && level.GetEntityAt(x, y, z) == nil
	}
	if standableFree(cx, cy, cz) {
		return cx, cy, cz
	}
	for r := 1; r < 40; r++ {
		for dx := -r; dx <= r; dx++ {
			for dy := -r; dy <= r; dy++ {
				if absI(dx) != r && absI(dy) != r {
					continue
				}
				if standableFree(cx+dx, cy+dy, cz) {
					return cx + dx, cy + dy, cz
				}
			}
		}
	}
	return cx, cy, cz
}

// Current returns the MainState of the location in orbit, or nil if none is loaded.
func (wm *WorldManager) Current() *MainState { return wm.loaded[wm.currentID] }

// NoSiteReason is the one-line reason the dashboard gives when there is no
// level to act on: adrift at the Wreck Site, or otherwise not in orbit.
func (wm *WorldManager) NoSiteReason() string {
	if wm.Campaign != nil && wm.Campaign.IsAdrift() {
		return "Adrift at the Wreck Site: nothing to land on. Travel to a world first."
	}
	return "Not in orbit — travel to a site from the Star Map first."
}

// addLoaded records ms as the loaded location id, most recently visited.
func (wm *WorldManager) addLoaded(id string, ms *MainState) {
	wm.loaded[id] = ms
	ms.locationID = id
	wm.visits++
	wm.lastVisit[id] = wm.visits
}

// setCurrent records ms as the loaded location id and puts it in orbit.
func (wm *WorldManager) setCurrent(id string, ms *MainState) {
	wm.addLoaded(id, ms)
	wm.currentID = id
}

// orbit makes ms's scenario the orbited one. Only entering orbit moves the global;
// a planet loaded any other way runs on ms.scenario and leaves it alone.
func orbit(ms *MainState) {
	if ms.scenario != nil {
		_ = scenario.SelectByID(ms.scenario.ID)
	}
}

// isStaffed reports whether a level has a colony worker to keep it simulated:
// colonists and robots count, hostile workers don't (decisions #3 and #4).
func isStaffed(s *MainState) bool {
	if s == nil || s.level == nil {
		return false
	}
	return countLevelColonists(s.level, s.colonyName()) > 0
}

// IsLoaded reports whether a location is in memory.
func (wm *WorldManager) IsLoaded(id string) bool { return wm.loaded[id] != nil }

// maxLoadedPlanets is the cap on planets in memory, not counting the ship. A planet
// costs 90-290 MB of heap; see docs/developer/background_simulation.md.
func maxLoadedPlanets() int {
	if n := config.Global().MaxLoadedPlanets; n > 0 {
		return n
	}
	return 3
}

// enforceLoadedCap freezes the least recently visited planets until the count is
// within the cap. The one in orbit is never evicted.
func (wm *WorldManager) enforceLoadedCap() error {
	for len(wm.loaded) > maxLoadedPlanets() {
		victim := ""
		for id := range wm.loaded {
			if id != wm.currentID && (victim == "" || wm.lastVisit[id] < wm.lastVisit[victim]) {
				victim = id
			}
		}
		if victim == "" {
			return nil
		}
		if err := wm.Freeze(wm.Campaign.Locations[victim], wm.loaded[victim]); err != nil {
			return err
		}
	}
	return nil
}

// TotalColonists counts colony workers everywhere: live on loaded planets, from the
// recorded tally on frozen ones, and aboard the ship.
func (wm *WorldManager) TotalColonists() int {
	n := len(wm.ShipColonists())
	for id, loc := range wm.Campaign.Locations {
		if ms := wm.loaded[id]; ms != nil {
			n += countLevelColonists(ms.level, ms.colonyName())
		} else {
			n += loc.Colonists
		}
	}
	return n
}

func NewWorldManager(c *campaign.Campaign) *WorldManager {
	wm := &WorldManager{
		Campaign:   c,
		loaded:     map[string]*MainState{},
		lastVisit:  map[string]int{},
		owed:       map[string]int{},
		lastDamage: map[string]int{},
	}
	if err := wm.buildShip(); err != nil {
		log.Printf("buildShip: %v", err)
	}
	return wm
}

// buildShip constructs The Ship's always-loaded MainState from the fixed
// template and parks it. The ship has no scenario (so gm.Update spawns no
// hostiles) and is never frozen while the campaign is active.
func (wm *WorldManager) buildShip() error {
	c := wm.Campaign
	shipName := wm.shipSettlementName()
	cfg := SettlementConfig{
		Name:         shipName,
		ColonyName:   shipName,
		CampaignMode: true,
		// The ship is an enclosed, powered hull — it lights itself, independent
		// of whatever location the campaign is orbiting.
		LightingMode:    "fixed",
		LightingAmbient: 100,
	}
	// Resume the saved ship if this campaign has one, else build a fresh hull.
	// The ship is its OWN settlement (distinct from the colony) so its build/task
	// queue doesn't collide with planet settlements in the global name-keyed
	// lookup; its storage is owned by that settlement. See the_ship.md.
	level, err := loadLocationLevel(wm.shipSaveFile())
	if err != nil || level == nil {
		level = ship.BuildShipLevel(shipName)
	}
	ms, err := newMainStateFromLevel(level, cfg)
	if err != nil {
		return err
	}
	ms.campaign = c
	ms.wm = wm
	ms.MainSettlement = wm.ensureShipSettlement(shipName)
	ms.gm.suppressSpawns = true // the ship is a safe haven; no hostile waves
	// Frame the camera on the starting hull — the level is mostly empty void.
	hx, hy, hz := ship.HullCenter()
	ms.camera.CenterOn(hx, hy, hz)
	ms.refreshResourceScanner()
	ms.storageInspector = newStorageInspectorModal(wm)
	ms.storageInspector.OnBeginRelocate = func(source *ecs.Entity, blueprint string, maxAvail int) {
		ms.BeginRelocate(source, blueprint, maxAvail)
	}
	ms.teardown() // parked until visited
	wm.shipLevel = ms
	return nil
}

// shipSettlementName is the ship's own settlement — distinct from the colony so
// its task queue and storage don't collide with planet settlements in the
// global, name-keyed settlement lookup.
func (wm *WorldManager) shipSettlementName() string {
	return campaignColonyName(wm.Campaign) + " Ship"
}

// ensureShipSettlement returns the ship's settlement, reusing the one restored
// from a save if present, else creating it centered on the hull.
func (wm *WorldManager) ensureShipSettlement(name string) *settlement.Settlement {
	if existing, ok := settlement.Settlements[name]; ok {
		return existing
	}
	hx, hy, hz := ship.HullCenter()
	s := settlement.NewSettlement(hx, hy, hz)
	delete(settlement.Settlements, s.Name) // drop the auto-generated name key
	s.Name = name
	settlement.Settlements[name] = s
	return s
}

// TickBackground runs one frame of background simulation, skipping whichever level
// is live (it ticks itself via stepWorld) and honoring the pause flag. The ship, which
// is cheap and always aboard, steps every frame. Staffed away planets each accrue one
// owed tick and share a per-frame budget round-robin, so a few run at real time and
// many degrade evenly. Owed ticks past the cap are dropped, not fast-forwarded later.
func (wm *WorldManager) TickBackground(live *MainState) {
	if wm.ClockStopped() {
		return
	}
	if wm.shipLevel != nil && wm.shipLevel != live {
		// The live level's Update reset the per-frame pathfinding budget for
		// itself; each background tick needs its own.
		fspath.ResetFrameCounter()
		wm.shipLevel.stepBackground()
	}
	cfg := config.Global()
	maxOwed := positiveOr(cfg.BackgroundOwedCap, 60)
	var away []string
	for id, ms := range wm.loaded {
		if ms == live {
			wm.owed[id] = 0 // it ticks itself now; no catching up after it leaves
			continue
		}
		if !isStaffed(ms) {
			// Crew lost while away: nothing keeps this planet simulated (decision
			// #4). The one in orbit stays in memory, ready for a beam-down.
			wm.owed[id] = 0
			if id != wm.currentID {
				if err := wm.Freeze(wm.Campaign.Locations[id], ms); err != nil {
					log.Printf("freeze unstaffed %s: %v", id, err)
				}
			}
			continue
		}
		if wm.owed[id] < maxOwed {
			wm.owed[id]++
		}
		away = append(away, id)
	}
	sort.Strings(away)
	wm.payOwedTicks(away, positiveOr(cfg.BackgroundTicksPerFrame, 2))
}

// payOwedTicks spends budget background ticks across the sorted planet IDs, one each
// in turn, resuming after the planet served last.
func (wm *WorldManager) payOwedTicks(away []string, budget int) {
	if len(away) == 0 {
		return
	}
	next := sort.Search(len(away), func(i int) bool { return away[i] > wm.lastServed })
	for idle := 0; budget > 0 && idle < len(away); next++ {
		id := away[next%len(away)]
		if wm.owed[id] == 0 {
			idle++
			continue
		}
		idle = 0
		wm.owed[id]--
		budget--
		wm.lastServed = id
		fspath.ResetFrameCounter()
		wm.loaded[id].stepBackground()
	}
}

func positiveOr(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// ClockStopped reports whether the whole world clock is stopped, by the pause
// toggle or by an incident awaiting an answer.
func (wm *WorldManager) ClockStopped() bool {
	return wm.paused || wm.ActiveIncident() != nil
}

// updatePopup runs the incident popup for a frame; see incidentPopup.update.
func (wm *WorldManager) updatePopup() state.StateInterface {
	if wm.popup == nil {
		wm.popup = newIncidentPopup(wm)
	}
	return wm.popup.update()
}

func (wm *WorldManager) drawPopup(screen *ebiten.Image) {
	if wm.popup != nil {
		wm.popup.draw(screen)
	}
}

// SetPaused toggles the world clock (background ticking); Paused reports it.
func (wm *WorldManager) SetPaused(p bool) {
	wm.paused = p
	wm.pauseReason = ""
}
func (wm *WorldManager) Paused() bool { return wm.paused }

// EnterShip re-attaches The Ship's parked MainState and returns it for the state
// machine — the ship-equivalent of EnterCurrent. The ship is never frozen, so it
// is always available.
func (wm *WorldManager) EnterShip() (*MainState, error) {
	if wm.shipLevel == nil {
		return nil, fmt.Errorf("the ship is not available")
	}
	// Clear the transition flags set when the ship last opened the Star Map,
	// else it would bounce straight back.
	wm.shipLevel.done = false
	wm.shipLevel.next = nil
	wm.shipLevel.reattach()
	installCampaignStorageHook(wm.Campaign)
	return wm.shipLevel, nil
}

// ShipLevel returns The Ship's level, or nil if it hasn't been built.
func (wm *WorldManager) ShipLevel() *world.Level {
	if wm.shipLevel == nil {
		return nil
	}
	return wm.shipLevel.level
}

// shipSaveFile is the on-disk path for the ship level (always loaded, so unlike
// locations it is written on every campaign save).
func (wm *WorldManager) shipSaveFile() string {
	return filepath.Join(campaignRoot(wm.Campaign.Name), "ship.json.gz")
}

// freezeShip writes the ship level to disk so its hold and (later) crew persist.
func (wm *WorldManager) freezeShip() error {
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return nil
	}
	lvl := wm.shipLevel.level
	sf := saveFile{
		Meta: SaveMeta{
			Name:     "ship",
			SavedAt:  time.Now(),
			MapSizeW: lvl.GetWidth(),
			MapSizeH: lvl.GetHeight(),
			MapSizeZ: lvl.GetDepth(),
		},
		WorldData: world.SaveLevel(lvl, wm.shipSettlementName()),
	}
	blob, err := marshalSaveFile(sf)
	if err != nil {
		return err
	}
	root := campaignRoot(wm.Campaign.Name)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	return os.WriteFile(wm.shipSaveFile(), blob, 0644)
}

// startingResources lists every resource blueprint placed in the ship hold at
// campaign start. Fuel is handled separately (caller-supplied quantity).
var startingResources = []string{
	"metal_ore",
	"crystal",
	"biomass",
	"stone",
	"radioactive_material",
}

const startingResourceQty = 100

// defaultHoldStock is the normal starting hold: startingResourceQty of each
// starting resource.
func defaultHoldStock() map[string]int {
	stock := make(map[string]int, len(startingResources))
	for _, bp := range startingResources {
		stock[bp] = startingResourceQty
	}
	return stock
}

// tutorialHoldMetal is the metal a guided first landing starts with: exactly
// what the "beam metal down" step asks for, so the second planet's locker can
// only be built from metal mined on the Training Ground.
const tutorialHoldMetal = 10

// tutorialHoldStock is the normal hold with the metal reduced.
func tutorialHoldStock() map[string]int {
	stock := defaultHoldStock()
	stock["metal_ore"] = tutorialHoldMetal
	return stock
}

// SeedNewCampaign seeds a brand-new expedition's research state. The crew is
// spawned onto the ship level via WorldManager.SeedShipCrew and the hold via
// StockShipLocker, both once the ship level exists.
func SeedNewCampaign(c *campaign.Campaign) {
	// Debug aid: pre-research everything if config.unlockAllResearch is set so
	// research-gated features (Encyclopedia, Global Inventory tiers, etc.) are
	// immediately available on a fresh campaign.
	if config.Global().UnlockAllResearch {
		for key := range research.AllTechs() {
			c.UnlockTech(key)
		}
	}
}

// shipBeamHoldEntity returns the storage container on the ship that should
// receive beamed/seeded cargo — the "teleporter" pad if one exists (so it acts
// as a staging area to sort from), otherwise the first storage container.
func (wm *WorldManager) shipBeamHoldEntity() *ecs.Entity {
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return nil
	}
	var first *ecs.Entity
	for _, e := range wm.shipLevel.level.Entities {
		if e == nil || !e.HasComponent(components.Storage) {
			continue
		}
		if e.Blueprint == "teleporter" {
			return e
		}
		if first == nil {
			first = e
		}
	}
	return first
}

// shipLocker returns the storage component of the ship's beam-hold container.
func (wm *WorldManager) shipLocker() *components.StorageComponent {
	e := wm.shipBeamHoldEntity()
	if e == nil {
		return nil
	}
	return e.GetComponent(components.Storage).(*components.StorageComponent)
}

// IsShipEntity reports whether e currently lives on the ship level.
func (wm *WorldManager) IsShipEntity(e *ecs.Entity) bool {
	if wm.shipLevel == nil || wm.shipLevel.level == nil || e == nil {
		return false
	}
	for _, se := range wm.shipLevel.level.Entities {
		if se == e {
			return true
		}
	}
	return false
}

// addToShipHold drops qty of a resource blueprint into the ship's storage and
// returns how many units actually landed.
func (wm *WorldManager) addToShipHold(blueprint string, qty int) int {
	if qty <= 0 {
		return 0
	}
	sc := wm.shipLocker()
	if sc == nil {
		return 0
	}
	e, err := factory.Create(blueprint, 0, 0, 0)
	if err != nil {
		return 0
	}
	if e.HasComponent(components.Material) {
		if mc := e.GetComponent(components.Material).(*components.MaterialComponent); mc.IsStackable() {
			mc.Quantity = qty
			if sc.AddItem(e) {
				return qty
			}
			// A rejected partial fit leaves the remainder on the item.
			return qty - mc.Quantity
		}
	}
	// Non-stackables (rations, gear) are one entity per unit: qty of them.
	for i := 0; i < qty; i++ {
		if i > 0 {
			if e, err = factory.Create(blueprint, 0, 0, 0); err != nil {
				return i
			}
		}
		if !sc.AddItem(e) {
			return i
		}
	}
	return qty
}

// startingRations is how many ration packs the ship begins with so the crew has
// food while planetside before food production is set up (needs tick in the
// background — see docs/developer/the_ship.md).
const startingRations = 20

// StockShipLocker fills the ship's storage with a starting fuel reserve, the
// given stock of each resource (nil means the normal defaultHoldStock), and a
// supply of rations. Call once after the ship is built.
func (wm *WorldManager) StockShipLocker(fuel int, stock map[string]int) {
	wm.addToShipHold("fuel", fuel)
	if stock == nil {
		stock = defaultHoldStock()
	}
	// Fixed order so the hold's stacks always line up the same way.
	for _, bp := range startingResources {
		if qty, ok := stock[bp]; ok {
			wm.addToShipHold(bp, qty)
		}
	}
	// Rations are Food items (one per slot), so add them individually.
	if sc := wm.shipLocker(); sc != nil {
		for i := 0; i < startingRations; i++ {
			if e, err := factory.Create("ration_pack", 0, 0, 0); err == nil {
				sc.AddItem(e)
			}
		}
	}
}

// applyQuestReward grants a completed quest's reward: fuel/resources into the
// ship hold, and any new systems charted by SpawnSystems.
func (wm *WorldManager) applyQuestReward(q *campaign.Quest) {
	c := wm.Campaign
	wm.addToShipHold("fuel", q.Reward.Fuel)
	for bp, n := range q.Reward.Resources {
		wm.addToShipHold(bp, n)
	}
	if q.Reward.SpawnSystems > 0 {
		for _, loc := range c.Expand(q.Reward.SpawnSystems) {
			message.PostMessage("Mission", "New system charted: "+loc.Name)
		}
	}
}

// beamPartyOntoLevel rebuilds roster colonists and places them at the plaza,
// tagging them for the colony. Shared by newGame (campaign start) and resume.
func beamPartyOntoLevel(level *world.Level, party []*world.SaveEntity, colonyName string, x, y, z int) {
	for i, se := range party {
		if se == nil {
			continue
		}
		colonist := world.RebuildLiveEntity(se)
		if colonist.HasComponent(rlcomponents.Position) {
			pc := colonist.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			pc.SetPosition(x+i-len(party)/2, y, z)
		}
		if !colonist.HasComponent(components.Settlement) {
			colonist.AddComponent(&components.SettlementComponent{Name: colonyName})
		} else {
			colonist.GetComponent(components.Settlement).(*components.SettlementComponent).Name = colonyName
		}
		if !colonist.HasComponent(components.Worker) {
			colonist.AddComponent(&components.WorkerComponent{SelfDefend: true})
		} else {
			wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
			wc.CurrentTask = nil
		}
		level.AddEntity(colonist)
	}
}

// addToSite drops qty units of blueprint into the first colony-owned storage
// container found on level. Returns an error if none exists yet — the player
// must build a storage locker before beaming resources down.
// addToSite deposits qty of blueprint into the colony's storage on level and
// returns how many actually landed. Stackable materials go in as one stack that
// AddItem splits across containers as room allows; non-stackables (rations,
// gear) are created and placed one entity at a time, since AddItem takes one
// entity per slot. The caller debits only the returned count, so nothing is
// lost when the site is short of room.
func addToSite(level *world.Level, colonyName, blueprint string, qty int) (int, error) {
	if qty <= 0 {
		return 0, nil
	}
	probe, err := factory.Create(blueprint, 0, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("beam: unknown resource %q", blueprint)
	}
	stackable := false
	if probe.HasComponent(components.Material) {
		stackable = probe.GetComponent(components.Material).(*components.MaterialComponent).IsStackable()
	}

	var lockers []*components.StorageComponent
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy == colonyName {
				lockers = append(lockers, sc)
			}
		}
	}
	if len(lockers) == 0 {
		return 0, fmt.Errorf("no storage locker on site — build one first")
	}
	accepting := 0
	for _, sc := range lockers {
		if sc.Accepts(probe) {
			accepting++
		}
	}
	if accepting == 0 {
		return 0, fmt.Errorf("no storage container on site accepts %s; build a Storage Locker", displayMaterialName(blueprint))
	}

	deposited := 0
	if stackable {
		item := probe
		mc := item.GetComponent(components.Material).(*components.MaterialComponent)
		mc.Quantity = qty
		for _, sc := range lockers {
			if !sc.Accepts(item) {
				continue
			}
			before := mc.Quantity
			if sc.AddItem(item) {
				deposited += before
				break
			}
			// A rejected partial fit leaves the remainder on the item.
			deposited += before - mc.Quantity
			if mc.Quantity <= 0 {
				break
			}
		}
	} else {
		item := probe
		for deposited < qty {
			placed := false
			for _, sc := range lockers {
				if sc.Accepts(item) && sc.AddItem(item) {
					placed = true
					break
				}
			}
			if !placed {
				break
			}
			deposited++
			if deposited < qty {
				if item, err = factory.Create(blueprint, 0, 0, 0); err != nil {
					break
				}
			}
		}
	}
	if deposited == 0 {
		return 0, fmt.Errorf("site storage is full; nothing beamed")
	}
	return deposited, nil
}

// ShipHoldEntity returns the ship's primary beam-hold storage container, or nil
// if none exists.
func (wm *WorldManager) ShipHoldEntity() *ecs.Entity {
	return wm.shipBeamHoldEntity()
}

// SiteHoldEntity returns the first colony-owned storage container on the loaded
// site — the beam-up source — or nil if no site is loaded or it has no storage.
func (wm *WorldManager) SiteHoldEntity() *ecs.Entity {
	if wm.Current() == nil || wm.Current().level == nil {
		return nil
	}
	colony := wm.currentColonyName()
	for _, ents := range [][]*ecs.Entity{wm.Current().level.Entities, wm.Current().level.StaticEntities} {
		for _, e := range ents {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			if e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == colony {
				return e
			}
		}
	}
	return nil
}

// BeamResourceDown moves qty of blueprint from the ship hold into the first
// colony-owned storage container at the current site.
func (wm *WorldManager) BeamResourceDown(blueprint string, qty int) error {
	if qty <= 0 {
		return nil
	}
	if wm.Current() == nil || wm.Current().level == nil {
		return fmt.Errorf("no location loaded")
	}
	colony := wm.currentColonyName()
	p := storage.ShipProvider{Level: wm.ShipLevel()}
	owners := []string{wm.shipSettlementName()}
	if have := storage.CountResource(p, owners, blueprint); have < qty {
		return fmt.Errorf("only %d %s in ship hold", have, blueprint)
	}
	deposited, err := addToSite(wm.Current().level, colony, blueprint, qty)
	if err != nil {
		return err
	}
	// Debit the hold only for what landed, so a short-of-room site never loses
	// cargo. A partial beam still reports the shortfall to the player.
	storage.Deduct(p, owners, map[string]int{blueprint: deposited})
	if deposited < qty {
		return fmt.Errorf("beamed %d of %d %s; site storage is full", deposited, qty, blueprint)
	}
	return nil
}

// BeamResourceUpFrom moves qty of blueprint from a specific storage container
// into the ship hold. Returns an error if the container is short.
func (wm *WorldManager) BeamResourceUpFrom(container *ecs.Entity, blueprint string, qty int) error {
	if qty <= 0 {
		return nil
	}
	if container == nil || !container.HasComponent(components.Storage) {
		return fmt.Errorf("not a storage container")
	}
	sc := container.GetComponent(components.Storage).(*components.StorageComponent)
	have := sc.CountResource(blueprint)
	if have < qty {
		return fmt.Errorf("only %d %s here", have, blueprint)
	}
	// Debit only what landed, so a full hold never loses cargo.
	landed := wm.addToShipHold(blueprint, qty)
	sc.DeductResource(blueprint, landed)
	if landed < qty {
		return fmt.Errorf("beamed %d of %d %s; ship hold is full or refused it", landed, qty, blueprint)
	}
	return nil
}

// countLevelColonists tallies living colony workers on a level (for total-wipe
// detection recorded onto the frozen location).
func countLevelColonists(level *world.Level, colony string) int {
	if level == nil {
		return 0
	}
	n := 0
	for _, e := range level.Entities {
		if components.IsColonyWorker(e, colony) {
			n++
		}
	}
	return n
}

// locationSettlements names the settlements a location's save carries: its colony
// and any faction settlements, which are named "<faction>:<locID>". Never the ship.
func locationSettlements(loc *campaign.Location, colony string) []string {
	names := []string{colony}
	for name := range settlement.Settlements {
		if strings.HasSuffix(name, ":"+loc.ID) {
			names = append(names, name)
		}
	}
	return names
}

// Freeze writes a location's level to its file, drops it from memory and takes its
// settlements out of the global map; a thaw restores them from the save. The level
// must not tick again afterwards. To save one that keeps running, use writeLocation.
func (wm *WorldManager) Freeze(loc *campaign.Location, s *MainState) error {
	if err := wm.writeLocation(loc, s); err != nil {
		return err
	}
	for _, name := range locationSettlements(loc, s.colonyName()) {
		delete(settlement.Settlements, name)
	}
	delete(wm.loaded, loc.ID)
	delete(wm.lastVisit, loc.ID)
	delete(wm.owed, loc.ID)
	if wm.currentID == loc.ID {
		wm.currentID = ""
	}
	return nil
}

// writeLocation serializes a location's level to its per-location file and records
// view state. The level stays live.
func (wm *WorldManager) writeLocation(loc *campaign.Location, s *MainState) error {
	c := wm.Campaign
	if loc == nil || s == nil || s.level == nil {
		return fmt.Errorf("freeze: no location")
	}
	colony := s.colonyName()

	loc.CameraX, loc.CameraY, loc.CameraZ = s.camera.X, s.camera.Y, s.camera.Z
	loc.BuildMode = s.buildMode
	loc.Visited = true
	loc.Colonists = countLevelColonists(s.level, colony)
	// Snapshot storage so the Global Inventory can read this site without
	// having to load its level file.
	loc.StorageSummary = storage.Summarize(
		storage.LevelProvider{Level: s.level},
		[]string{colony},
	)

	root := campaignRoot(c.Name)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	rel := "loc_" + sanitizeName(loc.ID) + ".json.gz"
	sf := saveFile{
		Meta: SaveMeta{
			Name:       loc.ID,
			ScenarioID: loc.ScenarioID,
			MapID:      loc.MapID,
			Seed:       loc.Seed,
			SavedAt:    time.Now(),
			MapSizeW:   s.level.GetWidth(),
			MapSizeH:   s.level.GetHeight(),
			MapSizeZ:   s.level.GetDepth(),
		},
		WorldData: world.SaveLevel(s.level, locationSettlements(loc, colony)...),
	}
	blob, err := marshalSaveFile(sf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, rel), blob, 0644); err != nil {
		return err
	}
	loc.SaveFile = rel
	if loc.SettlementName == "" {
		loc.SettlementName = colony
	}
	return nil
}

// buildParked loads (resumes) or generates a location's MainState WITHOUT
// entering it: listeners are detached so it can sit on the Star Map while the
// player shuffles colonists. Resume (EnterCurrent) re-attaches it.
func (wm *WorldManager) buildParked(locID string) (*MainState, error) {
	c := wm.Campaign
	loc := c.Locations[locID]
	if loc == nil {
		return nil, fmt.Errorf("activate: unknown location %q", locID)
	}
	colony := locationColonyName(c, loc)
	cfg := SettlementConfig{
		Name:         colony,
		ScenarioID:   loc.ScenarioID,
		MapID:        loc.MapID,
		Seed:         loc.Seed,
		ColonyName:   colony,
		CampaignMode: true,
	}

	var ms *MainState
	if loc.SaveFile != "" {
		level, err := loadLocationLevel(filepath.Join(campaignRoot(c.Name), loc.SaveFile))
		if err != nil {
			return nil, err
		}
		ms, err = newMainStateFromLevel(level, cfg)
		if err != nil {
			return nil, err
		}
		if ms.MainSettlement == nil {
			if mset, ok := settlement.Settlements[colony]; ok {
				ms.MainSettlement = mset
			}
		}
		if loc.CameraZ != 0 || loc.CameraX != 0 || loc.CameraY != 0 {
			ms.camera.X, ms.camera.Y, ms.camera.Z = loc.CameraX, loc.CameraY, loc.CameraZ
		}
		if loc.BuildMode != "" {
			ms.buildMode = loc.BuildMode
		}
	} else {
		var err error
		ms, err = NewMainState(cfg)
		if err != nil {
			return nil, err
		}
	}

	ms.campaign = c
	ms.wm = wm
	// Cache the generated map dimensions on the location so the Star Map can
	// size its icon without loading the level.
	if ms.level != nil {
		loc.MapW, loc.MapH = ms.level.GetWidth(), ms.level.GetHeight()
		// Capture a station's structure footprint for its Star Map icon
		// (recomputed if the stored grid resolution no longer matches).
		if loc.Kind == "station" && len(loc.StationFootprint) != stationFootprintDim {
			loc.StationFootprint = stationFootprint(ms.level)
		}
	}
	// The campaign (which holds researched techs in campaign mode) is only wired
	// up now, after MainState construction — re-sync anything that gates on it.
	ms.refreshResourceScanner()
	ms.storageInspector = newStorageInspectorModal(wm)
	ms.storageInspector.OnBeginRelocate = func(source *ecs.Entity, blueprint string, maxAvail int) {
		ms.BeginRelocate(source, blueprint, maxAvail)
	}
	loc.Visited = true
	spawnLocationFixtures(c, loc, ms.level)
	installCampaignStorageHook(c)
	// Park it: detach listeners until the player Resumes.
	ms.teardown()
	return ms, nil
}

// stationFootprint downsamples a station level's structure into a square
// occupancy grid (one row bitmask per entry: bit j of row i set = that region
// contains a hull deck). Used to draw a map-shaped Star Map icon.
func stationFootprint(level *world.Level) []uint16 {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	if w <= 0 || h <= 0 {
		return nil
	}
	rows := make([]uint16, stationFootprintDim)
	for oy := 0; oy < stationFootprintDim; oy++ {
		y0, y1 := oy*h/stationFootprintDim, (oy+1)*h/stationFootprintDim
		for ox := 0; ox < stationFootprintDim; ox++ {
			x0, x1 := ox*w/stationFootprintDim, (ox+1)*w/stationFootprintDim
			if regionHasDeck(level, x0, x1, y0, y1, d) {
				rows[oy] |= 1 << uint(ox)
			}
		}
	}
	return rows
}

// regionHasDeck reports whether any tile in the [x0,x1)×[y0,y1) column region
// (across all Z) has a Floor — i.e. a hull deck marks the station's extent.
func regionHasDeck(level *world.Level, x0, x1, y0, y1, d int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			for z := 0; z < d; z++ {
				if ti := level.GetTileAt(x, y, z); ti != nil {
					if t, ok := ti.(*world.Tile); ok && !t.Floor.IsEmpty() {
						return true
					}
				}
			}
		}
	}
	return false
}

// spawnLocationFixtures materializes this location's quest fixtures (named
// bosses / bounty creatures / loot) into the level. Idempotent: only fixtures
// not yet Spawned are created, so re-entering never duplicates them and a
// fixture added after the first visit appears on the next trip.
func spawnLocationFixtures(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	if loc == nil || level == nil {
		return
	}
	ax, ay, az := findStartingPlaza(level, level.SurfaceZ, 5)
	if ax == -1 {
		ax, ay, az = level.GetWidth()/2, level.GetHeight()/2, level.SurfaceZ
	}
	for i := range loc.Fixtures {
		f := &loc.Fixtures[i]
		if f.Spawned || (f.Blueprint == "" && f.Structure == "") {
			continue
		}
		// Scatter fixtures around the plaza (datapads widely, so finding them
		// means exploring); freeLandingTile spirals to the nearest standable,
		// unoccupied tile from there.
		h := fixtureHash(f.QuestID, i)
		var ox, oy int
		if f.Kind == "datapad" {
			ox = (h%81 - 40) * 3 // roughly ±120 tiles
			oy = ((h/81)%81 - 40) * 3
		} else {
			ox = h%25 - 12
			oy = (h/25)%25 - 12
		}
		x, y, z := freeLandingTile(level, ax+ox, ay+oy, az)

		// Structure fixtures delegate everything to the generator script: it
		// builds the structure AND spawns/names the quest target (and any
		// guards) inside it via spawn_quest_target / spawn_entity_named. We
		// only feed it the quest context as params.
		if f.Structure != "" {
			sw, sh := f.StructW, f.StructH
			if sw <= 0 {
				sw = 9
			}
			if sh <= 0 {
				sh = 7
			}
			ctx := &setupContext{
				Level:      level,
				rng:        rand.New(rand.NewSource(loc.Seed + int64(h))),
				bindTarget: func(qid, npc string) { c.BindQuestTargetName(qid, npc) },
			}
			p := ctx.topParams()
			p["quest_id"] = f.QuestID
			if err := runGenStructure(ctx, f.Structure, x-sw/2, y-sh/2, sw, sh); err != nil {
				log.Printf("[fixture] structure %q for quest %s: %v", f.Structure, f.QuestID, err)
			}
			f.Spawned = true
			log.Printf("[fixture] generated structure %q for quest %s at %s [%d,%d,%d]",
				f.Structure, f.QuestID, loc.Name, x, y, z)
			continue
		}

		e, err := factory.Create(f.Blueprint, x, y, z)
		if err != nil {
			continue
		}
		if f.Name != "" {
			if e.HasComponent(rlcomponents.Description) {
				e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name = f.Name
			} else {
				e.AddComponent(&rlcomponents.DescriptionComponent{Name: f.Name})
			}
		}
		if f.Kind == "datapad" {
			e.AddComponent(&components.DatapadComponent{QuestID: f.QuestID})
		} else {
			e.AddComponent(&components.QuestTargetComponent{QuestID: f.QuestID})
		}
		level.AddEntity(e)
		f.Spawned = true
		log.Printf("[fixture] spawned %s %q (%s) for quest %s at %s [%d,%d,%d]",
			f.Kind, f.Name, f.Blueprint, f.QuestID, loc.Name, x, y, z)
	}
}

func fixtureHash(id string, i int) int {
	h := i*2654435761 + 1
	for _, r := range id {
		h = h*16777619 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// Travel makes locID the current location, resuming or loading/generating it
// (parked). The location left behind stays in memory if staffed, else it is frozen
// to disk. Fuel/roster are handled by the caller. The Star Map stays open afterward.
func (wm *WorldManager) Travel(locID string) error {
	c := wm.Campaign
	dest := c.Locations[locID]
	if dest == nil {
		return fmt.Errorf("travel: unknown location %q", locID)
	}
	from := c.CurrentLocationID // origin of this jump, for the travel-route log
	if cur := wm.Current(); cur != nil {
		if c.CurrentLocationID == locID {
			return nil // already loaded
		}
		if isStaffed(cur) {
			wm.currentID = ""
		} else if err := wm.Freeze(c.Locations[wm.currentID], cur); err != nil {
			return err
		}
	}
	// Log the traversed route (both real, distinct locations) so the Star Map
	// can draw trade lines that darken with repeated use.
	c.RecordTravel(from, locID)
	// Reaching Home ends the run in victory — no level to generate or land.
	if dest.Kind == campaign.HomeKind {
		c.CurrentLocationID = locID
		c.Won = true
		return nil
	}
	// The Wreck Site is empty space: the ship sits there with no level, nothing
	// to orbit and nothing to land on. (The Star Map refuses the jump; this
	// keeps the state consistent if anything else asks.)
	if dest.Kind == campaign.AdriftKind {
		c.CurrentLocationID = locID
		wm.currentID = ""
		wm.landSet = false
		return nil
	}
	c.CurrentLocationID = locID
	wm.landSet = false
	ms := wm.loaded[locID]
	if ms != nil {
		// A fixture added while this planet stayed loaded appears on arrival,
		// as it would on a thaw.
		spawnLocationFixtures(c, dest, ms.level)
	} else {
		var err error
		if ms, err = wm.buildParked(locID); err != nil {
			return err
		}
	}
	wm.setCurrent(locID, ms)
	orbit(ms)
	if err := wm.enforceLoadedCap(); err != nil {
		return err
	}
	// Travelling stirs up new contracts: 1-in-6 chance of 1d4 fresh quests
	// (existing systems and/or newly charted ones) so the run keeps flowing.
	if newLocs, added := c.MaybeTravelQuests(); added > 0 {
		for _, loc := range newLocs {
			message.PostMessage("Mission", "New system charted: "+loc.Name)
		}
		message.PostMessage("Mission", fmt.Sprintf("New contracts received (%d).", added))
	}
	return nil
}

// EnterStartingOrbit parks the campaign's current location so a session begins
// already in orbit of it — no fuel cost, no travel-quest roll — rather than "in
// transit". No-op if a location is already loaded or the current site is Home.
func (wm *WorldManager) EnterStartingOrbit() error {
	c := wm.Campaign
	if wm.Current() != nil || c == nil || c.CurrentLocationID == "" {
		return nil
	}
	loc := c.Locations[c.CurrentLocationID]
	if !loc.HasSite() {
		// Home, or adrift at the Wreck Site: no level to park. Away planets
		// that were loaded still come back.
		wm.restoreLoaded()
		return nil
	}
	ms := wm.loaded[c.CurrentLocationID]
	if ms == nil {
		var err error
		if ms, err = wm.buildParked(c.CurrentLocationID); err != nil {
			return err
		}
	}
	wm.setCurrent(c.CurrentLocationID, ms)
	orbit(ms)
	wm.landSet = false
	wm.restoreLoaded()
	return nil
}

// restoreLoaded thaws the away planets that were in memory when the campaign was
// saved. One that fails to load stays frozen rather than blocking the campaign.
func (wm *WorldManager) restoreLoaded() {
	var ids []string
	for id, loc := range wm.Campaign.Locations {
		if loc.Loaded && wm.loaded[id] == nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		ms, err := wm.buildParked(id)
		if err != nil {
			log.Printf("restore %s: %v", id, err)
			continue
		}
		wm.addLoaded(id, ms)
	}
}

// EnterCurrent re-attaches the parked current location and returns it for the
// state machine to switch to (this is what Resume does).
func (wm *WorldManager) EnterCurrent() (*MainState, error) {
	if wm.Current() == nil {
		if wm.Campaign.CurrentLocationID == "" {
			return nil, fmt.Errorf("no location loaded — Travel first")
		}
		if wm.Campaign.IsAdrift() {
			return nil, fmt.Errorf("adrift at the Wreck Site: nothing to land on — Travel to a world first")
		}
		if err := wm.Travel(wm.Campaign.CurrentLocationID); err != nil {
			return nil, err
		}
	}
	// Clear the transition flags set when this MainState opened the Star Map,
	// otherwise it would immediately bounce back to the overworld.
	wm.Current().done = false
	wm.Current().next = nil
	wm.Current().reattach()
	installCampaignStorageHook(wm.Campaign)
	return wm.Current(), nil
}

// PlanetColonists returns the colonist entities on the currently-loaded level.
func (wm *WorldManager) PlanetColonists() []*ecs.Entity {
	if wm.Current() == nil || wm.Current().level == nil {
		return nil
	}
	colony := wm.Current().colonyName()
	var out []*ecs.Entity
	for _, e := range wm.Current().level.Entities {
		if components.IsColonyWorker(e, colony) {
			out = append(out, e)
		}
	}
	return out
}

// BeamDown moves one ship-roster colonist (by index) onto the loaded level near
// the landing plaza.
// ShipColonists returns the live worker entities aboard the ship — the roster.
func (wm *WorldManager) ShipColonists() []*ecs.Entity {
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return nil
	}
	colony := wm.shipSettlementName()
	var out []*ecs.Entity
	for _, e := range wm.shipLevel.level.Entities {
		if components.IsColonyWorker(e, colony) {
			out = append(out, e)
		}
	}
	return out
}

// tagColonistSettlement makes an entity a working colony member: cleared task,
// colony settlement, worker component.
func tagColonistSettlement(colonist *ecs.Entity, colony string) {
	if colonist.HasComponent(components.Settlement) {
		colonist.GetComponent(components.Settlement).(*components.SettlementComponent).Name = colony
	} else {
		colonist.AddComponent(&components.SettlementComponent{Name: colony})
	}
	if colonist.HasComponent(components.Worker) {
		colonist.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask = nil
	} else {
		colonist.AddComponent(&components.WorkerComponent{SelfDefend: true})
	}
}

// SeedShipCrew spawns n starting colonists as entities inside the ship's hull.
func (wm *WorldManager) SeedShipCrew(n int) {
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return
	}
	lvl := wm.shipLevel.level
	shipName := wm.shipSettlementName()
	cx, cy, cz := ship.HullCenter()
	for i := 0; i < n; i++ {
		ce, err := factory.Create("colonist", 0, 0, 0)
		if err != nil {
			continue
		}
		x, y, z := freeLandingTile(lvl, cx, cy, cz)
		if ce.HasComponent(rlcomponents.Position) {
			ce.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
		}
		tagColonistSettlement(ce, shipName)
		lvl.AddEntity(ce)
	}
}

// BeamDown transfers a specific ship colonist to the orbited planet's level.
// Takes the entity (not a roster index) so a background tick that reorders or
// removes crew between selection and beam can't send down the wrong colonist.
func (wm *WorldManager) BeamDown(colonist *ecs.Entity) error {
	if wm.Current() == nil || wm.Current().level == nil {
		return fmt.Errorf("no location loaded — Travel first")
	}
	if colonist == nil || !wm.IsShipEntity(colonist) {
		return fmt.Errorf("no colonist selected")
	}
	// Remove from the ship BEFORE repositioning: RemoveEntity clears the spatial
	// index at the entity's current position, so moving it first would leave a
	// stale occupant on the old ship tile.
	wm.shipLevel.level.RemoveEntity(colonist)
	level := wm.Current().level
	bx, by, bz := wm.landingZone()
	x, y, z := freeLandingTile(level, bx, by, bz)
	if colonist.HasComponent(rlcomponents.Position) {
		colonist.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	tagColonistSettlement(colonist, wm.currentColonyName())
	level.AddEntity(colonist)
	return nil
}

// BeamUp moves one colonist from the loaded level back to the ship roster.
func (wm *WorldManager) BeamUp(e *ecs.Entity) error {
	if wm.Current() == nil || wm.Current().level == nil {
		return fmt.Errorf("no location loaded")
	}
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return fmt.Errorf("the ship is not available")
	}
	if e == nil {
		return fmt.Errorf("beam up: no colonist")
	}
	if s := wm.Campaign.Ship; s != nil && s.RosterCap > 0 && len(wm.ShipColonists()) >= s.RosterCap {
		return fmt.Errorf("ship roster is full (%d/%d)", len(wm.ShipColonists()), s.RosterCap)
	}
	// No beaming through ground.
	if e.HasComponent(rlcomponents.Position) {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !campaign.OpenToSky(wm.Current().level, pc.GetX(), pc.GetY(), pc.GetZ()) {
			return fmt.Errorf("colonist is not open to the sky — no beaming through ground")
		}
	}
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			wc.CurrentTask.Stop()
		}
		wc.CurrentTask = nil
	}
	// Transfer the entity from the planet level to a free tile in the ship hull.
	wm.Current().level.RemoveEntity(e)
	shipLvl := wm.shipLevel.level
	cx, cy, cz := ship.HullCenter()
	x, y, z := freeLandingTile(shipLvl, cx, cy, cz)
	if e.HasComponent(rlcomponents.Position) {
		e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	tagColonistSettlement(e, wm.shipSettlementName()) // now a ship-settlement member
	shipLvl.AddEntity(e)
	return nil
}

func campaignColonyName(c *campaign.Campaign) string {
	if c.Name != "" {
		return c.Name + " Colony"
	}
	return "Colony Alpha"
}

// locationColonyName returns the location's own colony settlement name, assigning
// it on first use. Every planet needs its own: settlements live in a global map
// keyed by name, so a shared name would mean a shared task queue and storage owner.
func locationColonyName(c *campaign.Campaign, loc *campaign.Location) string {
	if loc.SettlementName == "" {
		loc.SettlementName = campaignColonyName(c) + " @ " + loc.Name
	}
	return loc.SettlementName
}

// currentColonyName is the colony settlement of the location in orbit.
func (wm *WorldManager) currentColonyName() string {
	loc := wm.Campaign.CurrentLocation()
	if loc == nil {
		return ""
	}
	return locationColonyName(wm.Campaign, loc)
}

// SaveCampaign writes every loaded location's level, the ship, and the campaign root
// file. Everything stays live. s is the level the player saved from (nil from the
// dashboard with nothing in orbit), which supplies the day.
func (wm *WorldManager) SaveCampaign(s *MainState) error {
	c := wm.Campaign
	if c == nil {
		return fmt.Errorf("save: no campaign")
	}
	if s != nil {
		c.Day = s.day
	}
	for id, loc := range c.Locations {
		ms := wm.loaded[id]
		loc.Loaded = ms != nil
		if ms != nil {
			if err := wm.writeLocation(loc, ms); err != nil {
				return err
			}
		}
	}
	if err := wm.freezeShip(); err != nil {
		return err
	}
	root := campaignRoot(c.Name)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	return writeGz(filepath.Join(root, "campaign.json.gz"), c)
}

// LoadCampaign reads a campaign root file (no level is loaded yet — the caller
// activates the current location).
func LoadCampaign(name string) (*campaign.Campaign, error) {
	var c campaign.Campaign
	if err := readGz(filepath.Join(campaignRoot(name), "campaign.json.gz"), &c); err != nil {
		return nil, err
	}
	// Saves from before per-location settlements tag everything on a planet with
	// the old shared colony name, so its workers would find no settlement and sit
	// idle. There is no migration; refuse them rather than load a dead colony.
	for _, loc := range c.Locations {
		if loc.SaveFile != "" && loc.SettlementName == "" {
			return nil, fmt.Errorf("campaign %q predates per-location settlements and can't be loaded", name)
		}
	}
	return &c, nil
}

// ListCampaigns returns the names of saved campaigns.
func ListCampaigns() []string {
	entries, err := os.ReadDir(campaignDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(campaignDir, e.Name(), "campaign.json.gz")); err == nil {
			out = append(out, e.Name())
		}
	}
	return out
}

func loadLocationLevel(path string) (*world.Level, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sf, err := unmarshalSaveFile(raw)
	if err != nil {
		return nil, err
	}
	return world.LoadSaveData(sf.WorldData), nil
}

// gzip helpers reused for the campaign root file.
func writeGz(path string, v any) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(v); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func readGz(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return json.Unmarshal(raw, v)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
