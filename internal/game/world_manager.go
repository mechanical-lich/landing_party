package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/ship"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
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
	// current is the loaded location's MainState. While the Star Map is open
	// it is "parked": kept in memory with its level intact but its event
	// listeners detached. Resume re-attaches and enters it.
	current *MainState

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
	ms := wm.current
	if ms != nil && ms.MainSettlement != nil {
		wm.landX, wm.landY, wm.landZ = ms.MainSettlement.CenterX, ms.MainSettlement.CenterY, ms.MainSettlement.CenterZ
	} else if ms != nil && ms.level != nil {
		x, y, z := findStartingPlaza(ms.level, ms.level.SurfaceZ, 5)
		if x == -1 {
			x, y, z = ms.CameraX, ms.CameraY, ms.CameraZ
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

func NewWorldManager(c *campaign.Campaign) *WorldManager {
	wm := &WorldManager{Campaign: c}
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
	cfg2 := config.Global()
	sidebarTiles := 200/ms.TileSizeW + 1
	viewW := cfg2.WorldWidth / ms.TileSizeW
	viewH := cfg2.WorldHeight / ms.TileSizeH
	ms.CameraX = hx - sidebarTiles - (viewW-sidebarTiles)/2
	ms.CameraY = hy - viewH/2
	ms.CameraZ = hz
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

// TickBackground advances the always-loaded background levels one step — for now
// just The Ship — skipping whichever level is currently live (it ticks itself
// via stepWorld) and honoring the pause flag.
func (wm *WorldManager) TickBackground(live *MainState) {
	if wm.paused {
		return
	}
	if wm.shipLevel != nil && wm.shipLevel != live {
		wm.shipLevel.stepBackground()
	}
}

// SetPaused toggles the world clock (background ticking); Paused reports it.
func (wm *WorldManager) SetPaused(p bool) { wm.paused = p }
func (wm *WorldManager) Paused() bool     { return wm.paused }

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
		WorldData: world.SaveLevel(lvl),
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

// addToShipHold drops qty of a resource blueprint into the ship's storage.
func (wm *WorldManager) addToShipHold(blueprint string, qty int) {
	if qty <= 0 {
		return
	}
	sc := wm.shipLocker()
	if sc == nil {
		return
	}
	if e, err := factory.Create(blueprint, 0, 0, 0); err == nil {
		if e.HasComponent(components.Material) {
			e.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
		}
		sc.AddItem(e)
	}
}

// startingRations is how many ration packs the ship begins with so the crew has
// food while planetside before food production is set up (needs tick in the
// background — see docs/developer/the_ship.md).
const startingRations = 20

// StockShipLocker fills the ship's storage with a starting fuel reserve, a base
// stock of each resource type, and a supply of rations. Call once after the ship
// is built.
func (wm *WorldManager) StockShipLocker(fuel int) {
	wm.addToShipHold("fuel", fuel)
	for _, bp := range startingResources {
		wm.addToShipHold(bp, startingResourceQty)
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
func addToSite(level *world.Level, colonyName, blueprint string, qty int) error {
	if qty <= 0 {
		return nil
	}
	// Build the item once so we can probe Accepts; we'll only consume it on
	// a successful deposit so a failed beam doesn't burn the entity.
	item, err := factory.Create(blueprint, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("beam: unknown resource %q", blueprint)
	}
	if item.HasComponent(components.Material) {
		item.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
	}
	hasAnyColonyLocker := false
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != colonyName {
				continue
			}
			hasAnyColonyLocker = true
			if !sc.Accepts(item) {
				continue
			}
			sc.AddItem(item)
			return nil
		}
	}
	if hasAnyColonyLocker {
		return fmt.Errorf("no storage container on site accepts %s", blueprint)
	}
	return fmt.Errorf("no storage locker on site — build one first")
}

// ShipHoldEntity returns the ship's primary beam-hold storage container, or nil
// if none exists.
func (wm *WorldManager) ShipHoldEntity() *ecs.Entity {
	return wm.shipBeamHoldEntity()
}

// BeamResourceDown moves qty of blueprint from the ship hold into the first
// colony-owned storage container at the current site.
func (wm *WorldManager) BeamResourceDown(blueprint string, qty int) error {
	if qty <= 0 {
		return nil
	}
	if wm.current == nil || wm.current.level == nil {
		return fmt.Errorf("no location loaded")
	}
	colony := campaignColonyName(wm.Campaign)
	p := storage.ShipProvider{Level: wm.ShipLevel()}
	owners := []string{wm.shipSettlementName()}
	if have := storage.CountResource(p, owners, blueprint); have < qty {
		return fmt.Errorf("only %d %s in ship hold", have, blueprint)
	}
	if err := addToSite(wm.current.level, colony, blueprint, qty); err != nil {
		return err
	}
	storage.Deduct(p, owners, map[string]int{blueprint: qty})
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
	sc.DeductResource(blueprint, qty)
	wm.addToShipHold(blueprint, qty)
	return nil
}

// Freeze serializes the current live level to its per-location file and records
// view state.
// countLevelColonists tallies living colonists on a level (for total-wipe
// detection recorded onto the frozen location).
func countLevelColonists(level *world.Level) int {
	if level == nil {
		return 0
	}
	n := 0
	for _, e := range level.Entities {
		if e != nil && e.HasComponent(components.Worker) && !e.HasComponent(rlcomponents.Dead) {
			n++
		}
	}
	return n
}

func (wm *WorldManager) Freeze(s *MainState) error {
	c := wm.Campaign
	loc := c.CurrentLocation()
	if loc == nil || s == nil || s.level == nil {
		return fmt.Errorf("freeze: no current location")
	}
	colony := s.settlementCfg.ColonyName
	if colony == "" {
		colony = s.settlementCfg.Name
	}

	loc.CameraX, loc.CameraY, loc.CameraZ = s.CameraX, s.CameraY, s.CameraZ
	loc.BuildMode = s.buildMode
	loc.Visited = true
	loc.Colonists = countLevelColonists(s.level)
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
		WorldData: world.SaveLevel(s.level),
	}
	blob, err := marshalSaveFile(sf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, rel), blob, 0644); err != nil {
		return err
	}
	loc.SaveFile = rel
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
	colony := campaignColonyName(c)
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
			ms.CameraX, ms.CameraY, ms.CameraZ = loc.CameraX, loc.CameraY, loc.CameraZ
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

// Travel makes locID the current location, loading/generating it (parked). Any
// previously-loaded different location is frozen to disk first. Fuel/roster are
// handled by the caller. The Star Map stays open afterward.
func (wm *WorldManager) Travel(locID string) error {
	c := wm.Campaign
	dest := c.Locations[locID]
	if dest == nil {
		return fmt.Errorf("travel: unknown location %q", locID)
	}
	if wm.current != nil {
		if c.CurrentLocationID == locID {
			return nil // already loaded
		}
		if err := wm.Freeze(wm.current); err != nil {
			return err
		}
		wm.current = nil
	}
	// Reaching Home ends the run in victory — no level to generate or land.
	if dest.Kind == campaign.HomeKind {
		c.CurrentLocationID = locID
		c.Won = true
		return nil
	}
	c.CurrentLocationID = locID
	wm.landSet = false
	ms, err := wm.buildParked(locID)
	if err != nil {
		return err
	}
	wm.current = ms
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

// EnterCurrent re-attaches the parked current location and returns it for the
// state machine to switch to (this is what Resume does).
func (wm *WorldManager) EnterCurrent() (*MainState, error) {
	if wm.current == nil {
		if wm.Campaign.CurrentLocationID == "" {
			return nil, fmt.Errorf("no location loaded — Travel first")
		}
		if err := wm.Travel(wm.Campaign.CurrentLocationID); err != nil {
			return nil, err
		}
	}
	// Clear the transition flags set when this MainState opened the Star Map,
	// otherwise it would immediately bounce back to the overworld.
	wm.current.done = false
	wm.current.next = nil
	wm.current.reattach()
	installCampaignStorageHook(wm.Campaign)
	return wm.current, nil
}

// PlanetColonists returns the colonist entities on the currently-loaded level.
func (wm *WorldManager) PlanetColonists() []*ecs.Entity {
	if wm.current == nil || wm.current.level == nil {
		return nil
	}
	var out []*ecs.Entity
	for _, e := range wm.current.level.Entities {
		if e != nil && e.HasComponent(components.Worker) && !e.HasComponent(rlcomponents.Dead) {
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
	var out []*ecs.Entity
	for _, e := range wm.shipLevel.level.Entities {
		if e != nil && e.HasComponent(components.Worker) && !e.HasComponent(rlcomponents.Dead) {
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

// BeamDown transfers the roster-index colonist from the ship level to the
// orbited planet's level.
func (wm *WorldManager) BeamDown(rosterIdx int) error {
	c := wm.Campaign
	if wm.current == nil || wm.current.level == nil {
		return fmt.Errorf("no location loaded — Travel first")
	}
	crew := wm.ShipColonists()
	if rosterIdx < 0 || rosterIdx >= len(crew) {
		return fmt.Errorf("no colonist selected")
	}
	colonist := crew[rosterIdx]
	// Remove from the ship BEFORE repositioning: RemoveEntity clears the spatial
	// index at the entity's current position, so moving it first would leave a
	// stale occupant on the old ship tile.
	wm.shipLevel.level.RemoveEntity(colonist)
	level := wm.current.level
	bx, by, bz := wm.landingZone()
	x, y, z := freeLandingTile(level, bx, by, bz)
	if colonist.HasComponent(rlcomponents.Position) {
		colonist.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	tagColonistSettlement(colonist, campaignColonyName(c))
	level.AddEntity(colonist)
	return nil
}

// BeamUp moves one colonist from the loaded level back to the ship roster.
func (wm *WorldManager) BeamUp(e *ecs.Entity) error {
	if wm.current == nil || wm.current.level == nil {
		return fmt.Errorf("no location loaded")
	}
	if wm.shipLevel == nil || wm.shipLevel.level == nil {
		return fmt.Errorf("the ship is not available")
	}
	if e == nil {
		return fmt.Errorf("beam up: no colonist")
	}
	if cap := wm.Campaign.Ship.RosterCap; cap > 0 && len(wm.ShipColonists()) >= cap {
		return fmt.Errorf("ship roster is full (%d/%d)", len(wm.ShipColonists()), cap)
	}
	// No beaming through ground.
	if e.HasComponent(rlcomponents.Position) {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !campaign.OpenToSky(wm.current.level, pc.GetX(), pc.GetY(), pc.GetZ()) {
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
	wm.current.level.RemoveEntity(e)
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

// SaveCampaign freezes the current level, flushes the ship hold, and writes the
// campaign root file. The per-location level files are written by Freeze.
func (wm *WorldManager) SaveCampaign(s *MainState) error {
	c := wm.Campaign
	if c == nil {
		return fmt.Errorf("save: no campaign")
	}
	if s != nil {
		if err := wm.Freeze(s); err != nil {
			return err
		}
		c.Day = s.day
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
