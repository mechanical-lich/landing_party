package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// OverworldState is the space "star map": pick a destination, load it, and
// shuffle colonists between the ship and the loaded planet before resuming
// play. Travel only loads/marks-current; Resume actually enters the level.
type OverworldState struct {
	campaign *campaign.Campaign
	wm       *WorldManager

	locList  *minui.ListBox
	locIDs   []string
	shipList *minui.ListBox
	planList *minui.ListBox
	planEnts []*ecs.Entity

	beamDownBtn      *minui.Button
	beamUpBtn        *minui.Button
	beamResBtn       *minui.Button
	globalInvBtn     *minui.Button
	storageInspector *StorageInspectorModal
	globalInventory  *GlobalInventoryModal
	travelBtn        *minui.Button
	resumeBtn        *minui.Button
	saveBtn          *minui.Button
	questBtn         *minui.Button

	status string
	done   bool
	next   state.StateInterface
}

func NewOverworldState(c *campaign.Campaign, wm *WorldManager) *OverworldState {
	o := &OverworldState{campaign: c, wm: wm}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	o.locList = minui.NewListBox("ow_locs", nil)
	o.locList.SetBounds(minui.Rect{X: cx - 300, Y: 180, Width: 600, Height: 150})
	o.locList.Layout() // compute visibleItems; otherwise only one row renders

	o.shipList = minui.NewListBox("ow_ship", nil)
	o.shipList.SetBounds(minui.Rect{X: cx - 300, Y: 388, Width: 250, Height: 210})
	o.shipList.Layout()

	o.planList = minui.NewListBox("ow_plan", nil)
	o.planList.SetBounds(minui.Rect{X: cx + 50, Y: 388, Width: 250, Height: 210})
	o.planList.Layout()

	o.beamDownBtn = minui.NewButton("ow_beam_down", "Beam Down  >")
	o.beamDownBtn.SetPosition(cx-44, 430)
	o.beamDownBtn.SetSize(90, 34)
	o.beamDownBtn.OnClick = func() { o.beamDown() }

	o.beamUpBtn = minui.NewButton("ow_beam_up", "<  Beam Up")
	o.beamUpBtn.SetPosition(cx-44, 480)
	o.beamUpBtn.SetSize(90, 34)
	o.beamUpBtn.OnClick = func() { o.beamUp() }

	// Star Map gets its own inspector instance, separate from MainState's.
	// We intentionally don't wire OnBeginRelocate here: the Star Map only opens
	// the ship hold (a ship-side source), and the inspector hides the Relocate
	// button on ship-side sources anyway — relocate is a per-site task that
	// only makes sense once a level is loaded.
	o.storageInspector = newStorageInspectorModal(wm)

	btnY := 624
	o.travelBtn = minui.NewButton("ow_travel", "Travel Here")
	o.travelBtn.SetPosition(cx-300, btnY)
	o.travelBtn.SetSize(180, 36)
	o.travelBtn.OnClick = func() { o.travel() }

	o.resumeBtn = minui.NewButton("ow_resume", "Resume / Land")
	o.resumeBtn.SetPosition(cx-90, btnY)
	o.resumeBtn.SetSize(180, 36)
	o.resumeBtn.OnClick = func() { o.resume() }

	o.beamResBtn = minui.NewButton("ow_beam_res", "Beam Resources...")
	o.beamResBtn.SetPosition(cx-150, btnY+46)
	o.beamResBtn.SetSize(132, 34)
	o.beamResBtn.OnClick = func() { o.openBeamResources() }

	o.globalInvBtn = minui.NewButton("ow_global_inv", "Global Inventory")
	o.globalInvBtn.SetPosition(cx+18, btnY+46)
	o.globalInvBtn.SetSize(150, 34)
	o.globalInvBtn.OnClick = func() { o.openGlobalInventory() }
	o.globalInventory = newGlobalInventoryModal(wm)

	o.saveBtn = minui.NewButton("ow_save", "Save Expedition")
	o.saveBtn.SetPosition(cx+120, btnY)
	o.saveBtn.SetSize(180, 36)
	o.saveBtn.OnClick = func() {
		if err := o.wm.SaveCampaign(o.wm.current); err != nil {
			o.status = "Save failed: " + err.Error()
		} else {
			o.status = "Expedition saved."
		}
	}

	o.questBtn = minui.NewButton("ow_quests", "Quest Log")
	o.questBtn.SetPosition(cx+170, 142)
	o.questBtn.SetSize(130, 30)
	o.questBtn.OnClick = func() {
		o.next = NewQuestLogState(o.campaign, o.wm)
		o.done = true
	}

	o.refreshLocations()
	o.refreshCrew()
	return o
}

func (o *OverworldState) refreshLocations() {
	o.locIDs = o.locIDs[:0]
	var labels []string
	sel := o.locList.SelectedIndex
	cfg := config.Global()
	for _, loc := range o.campaign.DiscoveredLocations() {
		o.locIDs = append(o.locIDs, loc.ID)
		tag := ""
		if loc.ID == o.campaign.CurrentLocationID && o.wm.current != nil {
			tag = "  [in orbit]"
		} else if loc.Visited {
			tag = "  [established]"
		}
		scenarioName := ""
		if cfg.DebugShowScenarioID {
			if s := scenario.ByID(loc.ScenarioID); s != nil {
				scenarioName = " · " + s.Name
			}
		}
		labels = append(labels, fmt.Sprintf("%s (%s%s) — fuel %d%s", loc.Name, loc.Kind, scenarioName, o.campaign.FuelCost(loc.ID), tag))
	}
	o.locList.SetItems(labels)
	if sel >= 0 && sel < len(labels) {
		o.locList.SelectedIndex = sel
	} else if len(labels) > 0 {
		o.locList.SelectedIndex = 0
	}
}

// refreshCrew rebuilds both the ship roster list and the loaded-planet list.
func (o *OverworldState) refreshCrew() {
	ship := make([]string, 0, len(o.campaign.Ship.Roster))
	for i, se := range o.campaign.Ship.Roster {
		ship = append(ship, fmt.Sprintf("%d. %s", i+1, colonistDisplayName(se)))
	}
	if len(ship) == 0 {
		ship = []string{"(no colonists aboard)"}
	}
	o.shipList.SetItems(ship)

	o.planEnts = o.wm.PlanetColonists()
	plan := make([]string, 0, len(o.planEnts))
	for i, e := range o.planEnts {
		plan = append(plan, fmt.Sprintf("%d. %s", i+1, entityDisplayName(e)))
	}
	if o.wm.current == nil {
		plan = []string{"(location not loaded — Travel Here)"}
	} else if len(plan) == 0 {
		plan = []string{"(nobody planetside)"}
	}
	o.planList.SetItems(plan)
}

func colonistDisplayName(se *world.SaveEntity) string {
	if se == nil {
		return "?"
	}
	return entityDisplayName(world.RebuildLiveEntity(se))
}

func entityDisplayName(e *ecs.Entity) string {
	if e == nil {
		return "Colonist"
	}
	name := "Colonist"
	if e.HasComponent(rlcomponents.Description) {
		if dc, ok := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent); ok && dc.Name != "" {
			name = dc.Name
		}
	}
	if e.HasComponent(components.StatProgression) {
		prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
		name += fmt.Sprintf("  [S:%d D:%d I:%d]", prog.Str.Level, prog.Dex.Level, prog.Int.Level)
	}
	return name
}

func (o *OverworldState) fuelAvailable() int {
	return storage.CountResource(
		storage.ShipProvider{Ship: o.campaign.Ship},
		[]string{campaign.ShipSettlementName},
		"fuel",
	)
}

func (o *OverworldState) selectedLocation() *campaign.Location {
	i := o.locList.SelectedIndex
	if i < 0 || i >= len(o.locIDs) {
		return nil
	}
	return o.campaign.Locations[o.locIDs[i]]
}

func (o *OverworldState) openBeamResources() {
	if o.wm.current == nil {
		o.status = "Travel to a location before beaming resources."
		return
	}
	hold := o.wm.ShipHoldEntity()
	if hold == nil {
		o.status = "Ship hold not initialised."
		return
	}
	o.storageInspector.Open(hold)
}

func (o *OverworldState) openGlobalInventory() {
	if !o.campaign.HasTech(techGlobalInvCurrent) {
		o.status = "Requires Inventory Survey research."
		return
	}
	o.globalInventory.Open()
}

func (o *OverworldState) beamDown() {
	if o.wm.current == nil {
		o.status = "Travel to a location before beaming down."
		return
	}
	idx := o.shipList.SelectedIndex
	if idx < 0 || idx >= len(o.campaign.Ship.Roster) {
		o.status = "Select a colonist on the ship."
		return
	}
	if err := o.wm.BeamDown(idx); err != nil {
		o.status = err.Error()
		return
	}
	o.status = "Colonist beamed down."
	o.refreshCrew()
}

func (o *OverworldState) beamUp() {
	idx := o.planList.SelectedIndex
	if idx < 0 || idx >= len(o.planEnts) {
		o.status = "Select a colonist planetside."
		return
	}
	if err := o.wm.BeamUp(o.planEnts[idx]); err != nil {
		o.status = err.Error()
		return
	}
	o.status = "Colonist beamed up to the ship."
	o.refreshCrew()
}

// travel loads/marks the selected location as current (spending fuel if the
// ship is actually relocating). It does NOT enter the level — Resume does.
func (o *OverworldState) travel() {
	loc := o.selectedLocation()
	if loc == nil {
		return
	}
	relocating := loc.ID != o.campaign.CurrentLocationID || o.wm.current == nil
	moving := loc.ID != o.campaign.CurrentLocationID
	if loc.ID == o.campaign.CurrentLocationID && o.wm.current != nil {
		o.status = "Already in orbit here. Shuffle colonists, then Resume / Land."
		return
	}
	cost := o.campaign.FuelCost(loc.ID) // capture before Travel changes "current"
	if moving {
		if fuel := o.fuelAvailable(); fuel < cost {
			o.status = fmt.Sprintf("Not enough fuel: have %d, need %d.", fuel, cost)
			return
		}
	}
	if err := o.wm.Travel(loc.ID); err != nil {
		o.status = "Travel failed: " + err.Error()
		return
	}
	if moving && cost > 0 {
		storage.Deduct(
			storage.ShipProvider{Ship: o.campaign.Ship},
			[]string{campaign.ShipSettlementName},
			map[string]int{"fuel": cost},
		)
	}
	if o.campaign.Won {
		o.next = NewCampaignEndState(true, fmt.Sprintf("The colonists are home at last. (Day %d)", o.campaign.Day))
		o.done = true
		return
	}
	if relocating {
		o.status = fmt.Sprintf("In orbit over %s. Beam colonists down, then Resume / Land.", loc.Name)
	}
	o.refreshLocations()
	o.refreshCrew()
}

func (o *OverworldState) resume() {
	ms, err := o.wm.EnterCurrent()
	if err != nil {
		o.status = "Resume failed: " + err.Error()
		return
	}
	o.next = ms
	o.done = true
}

func (o *OverworldState) Update() state.StateInterface {
	// Modals take full input priority while open.
	if o.storageInspector.Visible {
		o.storageInspector.Update()
		return o.next
	}
	if o.globalInventory.Visible {
		o.globalInventory.Update()
		return o.next
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && o.wm.current != nil {
		o.resume()
		return o.next
	}
	// Gate the Global Inventory button on the Tier 1 research; greys out
	// until "Inventory Survey" is researched so the player sees the feature
	// exists but knows it's locked.
	o.globalInvBtn.SetEnabled(o.campaign.HasTech(techGlobalInvCurrent))

	o.locList.Update()
	o.shipList.Update()
	o.planList.Update()
	o.beamDownBtn.Update()
	o.beamUpBtn.Update()
	o.beamResBtn.Update()
	o.globalInvBtn.Update()
	o.travelBtn.Update()
	o.resumeBtn.Update()
	o.saveBtn.Update()
	o.questBtn.Update()
	return o.next
}

func (o *OverworldState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{6, 8, 16, 255})
	cx := cfg.ScreenWidth / 2
	lbl := color.RGBA{160, 185, 215, 255}

	title := "Star Map"
	mlge_text.Draw(screen, title, 40, cx-len(title)*40*3/10/2, 90, color.RGBA{120, 200, 255, 255})

	hdr := fmt.Sprintf("Fuel: %d", o.fuelAvailable())
	mlge_text.Draw(screen, hdr, 16, cx-300, 150, color.RGBA{200, 220, 255, 255})

	if loc := o.selectedLocation(); loc != nil {
		mlge_text.Draw(screen, loc.Summary, 13, cx-300, 338, color.RGBA{170, 190, 210, 255})
		if loc.Visited {
			if s := scenario.ByID(loc.ScenarioID); s != nil {
				mlge_text.Draw(screen, s.Name+": "+s.Description, 13, cx-300, 354, color.RGBA{120, 200, 255, 255})
			}
		}
	}

	mlge_text.Draw(screen, "On Ship", 15, cx-300, 372, lbl)
	planTitle := "Planetside"
	if loc := o.campaign.CurrentLocation(); loc != nil && o.wm.current != nil {
		planTitle = "On " + loc.Name
	}
	mlge_text.Draw(screen, planTitle, 15, cx+50, 372, lbl)

	o.locList.Draw(screen)
	o.shipList.Draw(screen)
	o.planList.Draw(screen)
	o.beamDownBtn.Draw(screen)
	o.beamUpBtn.Draw(screen)
	o.beamResBtn.Draw(screen)
	o.globalInvBtn.Draw(screen)
	o.travelBtn.Draw(screen)
	o.resumeBtn.Draw(screen)
	o.saveBtn.Draw(screen)
	o.questBtn.Draw(screen)

	if o.status != "" {
		mlge_text.Draw(screen, o.status, 13, cx-300, cfg.ScreenHeight-40, color.RGBA{230, 160, 90, 255})
	}
	o.storageInspector.Draw(screen)
	o.globalInventory.Draw(screen)
	minui.FlushOverlays(screen)
}

func (o *OverworldState) Done() bool { return o.done }
