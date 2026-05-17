package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mechanical-lich/landing_party/internal/ai"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// installCampaignStorageHook makes worker crafting/fuel draw from both the live
// level and the campaign ship hold (logistics: ship hold + local).
func installCampaignStorageHook(c *campaign.Campaign) {
	ai.StorageProviderFor = func(level *world.Level, settlementName string) (storage.Provider, []string) {
		return storage.MultiProvider{Providers: []storage.Provider{
			storage.LevelProvider{Level: level},
			storage.ShipProvider{Ship: c.Ship},
		}}, []string{settlementName, campaign.ShipSettlementName}
	}
}

// campaignFuelProvider returns a provider+owners for reading/spending fuel from
// the ship hold (and, when a level is live, on-planet colony stock too).
func campaignFuelProvider(c *campaign.Campaign, level *world.Level, colony string) (storage.Provider, []string) {
	providers := []storage.Provider{storage.ShipProvider{Ship: c.Ship}}
	if level != nil {
		providers = append(providers, storage.LevelProvider{Level: level})
	}
	return storage.MultiProvider{Providers: providers}, []string{colony, campaign.ShipSettlementName}
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

func NewWorldManager(c *campaign.Campaign) *WorldManager { return &WorldManager{Campaign: c} }

// SeedNewCampaign stocks a brand-new expedition: the ship begins with a fuel
// reserve in the hold and a small crew aboard so the player can make the first
// planetfall immediately.
func SeedNewCampaign(c *campaign.Campaign, colonists, fuel int) {
	if fuel > 0 {
		if hold := c.Ship.LiveHold(); len(hold) > 0 {
			sc := hold[0].GetComponent(components.Storage).(*components.StorageComponent)
			if fe, err := factory.Create("fuel", 0, 0, 0); err == nil {
				if fe.HasComponent(components.ResourceItem) {
					fe.GetComponent(components.ResourceItem).(*components.ResourceItemComponent).Quantity = fuel
				}
				sc.AddItem(fe)
			}
		}
	}
	for i := 0; i < colonists; i++ {
		if ce, err := factory.Create("colonist", 0, 0, 0); err == nil {
			c.Ship.Roster = append(c.Ship.Roster, world.EntityToSaveEntity(ce))
		}
	}
	c.Ship.Sync()
}

// addToHold drops `qty` of a resource blueprint into the ship hold.
func addToHold(c *campaign.Campaign, blueprint string, qty int) {
	if qty <= 0 {
		return
	}
	hold := c.Ship.LiveHold()
	if len(hold) == 0 {
		return
	}
	sc := hold[0].GetComponent(components.Storage).(*components.StorageComponent)
	if e, err := factory.Create(blueprint, 0, 0, 0); err == nil {
		if e.HasComponent(components.ResourceItem) {
			e.GetComponent(components.ResourceItem).(*components.ResourceItemComponent).Quantity = qty
		}
		sc.AddItem(e)
	}
}

// applyQuestReward grants a completed quest's reward: fuel/resources into the
// ship hold, and reveals any locations (explicit reward list or a location
// whose RevealQuest names this quest).
func (wm *WorldManager) applyQuestReward(q *campaign.Quest) {
	c := wm.Campaign
	addToHold(c, "fuel", q.Reward.Fuel)
	for bp, n := range q.Reward.Resources {
		addToHold(c, bp, n)
	}
	reveal := func(id string) {
		if loc := c.Locations[id]; loc != nil && !loc.Discovered {
			loc.Discovered = true
			message.AddMessage("New location discovered: " + loc.Name)
		}
	}
	for _, id := range q.Reward.RevealLocations {
		reveal(id)
	}
	for id, loc := range c.Locations {
		if loc.RevealQuest == q.ID {
			reveal(id)
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

// sweepResourcesToShip moves every ResourceItem stack out of colony-owned
// storage on the level and into the ship hold — "all resources gathered end up
// on the ship". Called on Freeze so the stockpile is available campaign-wide.
func sweepResourcesToShip(level *world.Level, colonyName string, ship *campaign.ShipState) {
	hold := ship.LiveHold()
	if len(hold) == 0 {
		return
	}
	holdSC := hold[0].GetComponent(components.Storage).(*components.StorageComponent)
	move := func(entities []*ecs.Entity) {
		for _, e := range entities {
			if !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != colonyName {
				continue
			}
			kept := sc.Items[:0]
			for _, item := range sc.Items {
				if item != nil && item.HasComponent(components.ResourceItem) {
					holdSC.AddItem(item)
				} else {
					kept = append(kept, item)
				}
			}
			sc.Items = kept
		}
	}
	move(level.Entities)
	move(level.StaticEntities)
}

// Freeze serializes the current live level to its per-location file and records
// view state, after sweeping resources up to the ship.
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
	sweepResourcesToShip(s.level, colony, c.Ship)

	loc.CameraX, loc.CameraY, loc.CameraZ = s.CameraX, s.CameraY, s.CameraZ
	loc.BuildMode = s.buildMode
	loc.Visited = true

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
	loc.Visited = true
	installCampaignStorageHook(c)
	// Park it: detach listeners until the player Resumes.
	ms.teardown()
	return ms, nil
}

// Travel makes locID the current location, loading/generating it (parked). Any
// previously-loaded different location is frozen to disk first. Fuel/roster are
// handled by the caller. The Star Map stays open afterward.
func (wm *WorldManager) Travel(locID string) error {
	c := wm.Campaign
	if c.Locations[locID] == nil {
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
	c.CurrentLocationID = locID
	wm.landSet = false
	ms, err := wm.buildParked(locID)
	if err != nil {
		return err
	}
	wm.current = ms
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
func (wm *WorldManager) BeamDown(rosterIdx int) error {
	c := wm.Campaign
	if wm.current == nil || wm.current.level == nil {
		return fmt.Errorf("no location loaded — Travel first")
	}
	if rosterIdx < 0 || rosterIdx >= len(c.Ship.Roster) {
		return fmt.Errorf("no colonist selected")
	}
	se := c.Ship.Roster[rosterIdx]
	level := wm.current.level
	bx, by, bz := wm.landingZone()
	x, y, z := freeLandingTile(level, bx, by, bz)

	colonist := world.RebuildLiveEntity(se)
	if colonist == nil {
		return fmt.Errorf("could not rebuild colonist")
	}
	if colonist.HasComponent(rlcomponents.Position) {
		colonist.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	colony := campaignColonyName(c)
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
	level.AddEntity(colonist)
	c.Ship.Roster = append(c.Ship.Roster[:rosterIdx], c.Ship.Roster[rosterIdx+1:]...)
	return nil
}

// BeamUp moves one colonist from the loaded level back to the ship roster.
func (wm *WorldManager) BeamUp(e *ecs.Entity) error {
	c := wm.Campaign
	if wm.current == nil || wm.current.level == nil {
		return fmt.Errorf("no location loaded")
	}
	if e != nil && e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			wc.CurrentTask.Stop()
		}
		wc.CurrentTask = nil
	}
	return campaign.BeamUp(wm.current.level, c.Ship, e)
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
	c.Ship.Sync()
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
