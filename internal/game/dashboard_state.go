package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// dashContentTop is the Y below which tab panels draw their content; above it is
// the tab bar + info line.
const dashContentTop = 78

// dashboardPanel is one tab's content. It draws inside the content region below
// the tab bar, updates while it's the active tab, and may request a full state
// transition (e.g. the Star Map opening the classic screen, or Resume/Land)
// which the Dashboard performs on its behalf.
type dashboardPanel interface {
	Enter()                           // called when the tab becomes active — refresh data
	Update()                          // per-frame while active
	Draw(screen *ebiten.Image)        // draw the content region
	Transition() state.StateInterface // non-nil ⇒ Dashboard swaps to it, then clears it
}

type dashTab struct {
	id    string
	label string
	panel dashboardPanel
}

// DashboardState is the campaign hub, rendered as a tabbed workspace: a tab bar
// switches between Star Map, Crew, Global Inventory, Quests, and Encyclopedia
// panels, with Return-to-Ship/Site and Save as persistent toolbar actions. It
// replaces the overloaded single-screen overworld as the in-game entry point.
type DashboardState struct {
	campaign *campaign.Campaign
	wm       *WorldManager

	tabs   []dashTab
	tabBar *minui.TabBar
	active int

	returnShipBtn *minui.Button
	returnSiteBtn *minui.Button
	saveBtn       *minui.Button

	// storageInspector is the shared beam-resources modal, hosted here so it
	// overlays the whole dashboard; the Inventory tab opens it.
	storageInspector *StorageInspectorModal
	inspectorWasOpen bool

	status string
	done   bool
	next   state.StateInterface
}

func NewDashboardState(c *campaign.Campaign, wm *WorldManager) *DashboardState {
	d := &DashboardState{campaign: c, wm: wm}
	d.storageInspector = newStorageInspectorModal(wm)

	panels := []struct {
		id, label string
		panel     dashboardPanel
	}{
		{"starmap", "Star Map", newStarMapPanel(c, wm)},
		{"crew", "Crew", newCrewPanel(c, wm)},
		{"inventory", "Global Inventory", newGlobalInvPanel(c, wm, d.storageInspector)},
		{"quests", "Quests", newQuestsPanel(c, wm)},
		{"encyclopedia", "Encyclopedia", newEncyclopediaPanel(c, wm)},
	}

	cfg := config.Global()
	sw := cfg.ScreenWidth
	d.tabBar = minui.NewTabBar("dash_tabs", minui.Rect{X: 16, Y: 6, Width: sw - 20, Height: 40}, minui.TabsTop)
	d.tabBar.Style.ActiveFill = color.RGBA{6, 8, 16, 255} // = page bg, so the active tab opens into the page
	d.tabBar.Style.FontSize = 15
	for _, p := range panels {
		d.tabs = append(d.tabs, dashTab{id: p.id, label: p.label, panel: p.panel})
		d.tabBar.AddTab(p.id, p.label)
	}
	d.tabBar.OnChange = func(id string) { d.selectTabByID(id) }

	d.saveBtn = minui.NewButton("dash_save", "Save")
	d.saveBtn.SetPosition(sw-110, 8)
	d.saveBtn.SetSize(90, 34)
	d.saveBtn.OnClick = d.save

	d.returnShipBtn = minui.NewButton("dash_return_ship", "To Ship")
	d.returnShipBtn.SetPosition(sw-110-150, 8)
	d.returnShipBtn.SetSize(140, 34)
	d.returnShipBtn.OnClick = d.returnToShip

	d.returnSiteBtn = minui.NewButton("dash_return_site", "To Site")
	d.returnSiteBtn.SetPosition(sw-110-150-150, 8)
	d.returnSiteBtn.SetSize(140, 34)
	d.returnSiteBtn.OnClick = d.returnToSite

	if len(d.tabs) > 0 {
		d.tabs[0].panel.Enter()
	}
	return d
}

func (d *DashboardState) selectTab(i int) {
	if i < 0 || i >= len(d.tabs) || i == d.active {
		return
	}
	d.active = i
	d.status = ""
	d.tabs[i].panel.Enter()
}

func (d *DashboardState) selectTabByID(id string) {
	for i := range d.tabs {
		if d.tabs[i].id == id {
			d.selectTab(i)
			return
		}
	}
}

func (d *DashboardState) returnToShip() {
	ms, err := d.wm.EnterShip()
	if err != nil {
		d.status = "Board failed: " + err.Error()
		return
	}
	d.next = ms
	d.done = true
}

func (d *DashboardState) returnToSite() {
	if d.wm.current == nil {
		d.status = "Not in orbit — travel to a site from the Star Map first."
		return
	}
	ms, err := d.wm.EnterCurrent()
	if err != nil {
		d.status = "Resume failed: " + err.Error()
		return
	}
	d.next = ms
	d.done = true
}

func (d *DashboardState) save() {
	if err := d.wm.SaveCampaign(d.wm.current); err != nil {
		d.status = "Save failed: " + err.Error()
	} else {
		d.status = "Expedition saved."
		if d.wm.current != nil {
			d.wm.current.saveNamesDirty = true // refresh the HUD list on return
		}
	}
}

func (d *DashboardState) fuelAvailable() int {
	return storage.CountResource(
		storage.ShipProvider{Level: d.wm.ShipLevel()},
		[]string{d.wm.shipSettlementName()},
		"fuel",
	)
}

func (d *DashboardState) Update() state.StateInterface {
	if d.wm != nil {
		d.wm.TickBackground(nil)
	}
	// The beam-resources modal takes full input priority while open.
	if d.storageInspector.Visible {
		d.storageInspector.Update()
		d.inspectorWasOpen = true
		return d.next
	}
	if d.inspectorWasOpen {
		// Modal just closed — refresh the active panel so beamed totals update.
		d.inspectorWasOpen = false
		if d.active >= 0 && d.active < len(d.tabs) {
			d.tabs[d.active].panel.Enter()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if d.wm.current != nil {
			d.returnToSite()
		} else {
			d.returnToShip()
		}
		return d.next
	}

	d.tabBar.Update()
	d.returnSiteBtn.SetEnabled(d.wm.current != nil)
	d.returnShipBtn.Update()
	d.returnSiteBtn.Update()
	d.saveBtn.Update()

	if d.active >= 0 && d.active < len(d.tabs) {
		p := d.tabs[d.active].panel
		p.Update()
		if t := p.Transition(); t != nil {
			d.next = t
			d.done = true
		}
	}
	return d.next
}

func (d *DashboardState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{6, 8, 16, 255})

	d.tabBar.Draw(screen)
	d.returnShipBtn.Draw(screen)
	d.returnSiteBtn.Draw(screen)
	d.saveBtn.Draw(screen)

	site := "in transit"
	if loc := d.campaign.CurrentLocation(); loc != nil && d.wm.current != nil {
		site = "in orbit: " + loc.Name
	}
	info := fmt.Sprintf("Fuel: %d      Day: %d      %s", d.fuelAvailable(), d.campaign.Day, site)
	mlge_text.Draw(screen, info, 14, 20, 56, color.RGBA{160, 185, 215, 255})

	if d.active >= 0 && d.active < len(d.tabs) {
		d.tabs[d.active].panel.Draw(screen)
	}

	if d.status != "" {
		mlge_text.Draw(screen, d.status, 13, 20, cfg.ScreenHeight-30, color.RGBA{230, 160, 90, 255})
	}
	d.storageInspector.Draw(screen)
	minui.FlushOverlays(screen)
}

func (d *DashboardState) Done() bool { return d.done }
