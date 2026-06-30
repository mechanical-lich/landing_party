package game

import (
	"fmt"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Tech gate keys read by the Global Inventory modal.
const (
	techGlobalInvCurrent  = "global_inv_current"
	techGlobalInvAll      = "global_inv_all"
	techGlobalInvDetailed = "global_inv_detailed"
)

// Layout constants. Same "modal_h = last_y + h + title_bar + bottom_pad"
// derivation as the storage inspector so children don't overhang.
const (
	giModalW    = 720
	giTitleBarH = 30
	giBottomPad = 12

	giTabRowY  = 6
	giTabBtnH  = 28
	giTabBtnW  = 140
	giTabGap   = 6
	giSideX    = 8
	giBodyY    = giTabRowY + giTabBtnH + 8
	giBodyH    = 380
	giCloseY   = giBodyY + giBodyH + 10
	giCloseBtnW = 100
	giCloseBtnH = 28

	giModalH = giCloseY + giCloseBtnH + giTitleBarH + giBottomPad

	// "By Site" split: site list on the left, materials on the right.
	giSiteListW = 200
	giMaterialX = giSideX + giSiteListW + 12
	giMaterialW = giModalW - giMaterialX - giSideX
)

// GlobalInventoryModal is the Star Map dashboard that aggregates storage
// across the campaign. Tabs are gated by tech: Current Site → All Sites →
// By Site. View-only — transfers go through the Storage Inspector.
type GlobalInventoryModal struct {
	modal   *minui.Modal
	wm      *WorldManager
	Visible bool

	activeTab       string // "current" | "all" | "site"
	selectedSiteKey string // "ship" | location ID — for the By Site tab

	// Held across rebuilds so OnSelect handlers can resolve indices.
	siteKeys []string
}

func newGlobalInventoryModal(wm *WorldManager) *GlobalInventoryModal {
	return &GlobalInventoryModal{wm: wm}
}

// Open shows the modal in whichever tab the player's highest unlocked tech
// allows. Returns silently if no tier has been researched (the calling button
// is responsible for the gate, but the modal double-checks).
func (g *GlobalInventoryModal) Open() {
	c := g.wm.Campaign
	if c == nil || !c.HasTech(techGlobalInvCurrent) {
		return
	}
	switch {
	case c.HasTech(techGlobalInvDetailed):
		g.activeTab = "site"
	case c.HasTech(techGlobalInvAll):
		g.activeTab = "all"
	default:
		g.activeTab = "current"
	}
	g.selectedSiteKey = ""
	g.build()
	g.Visible = true
}

func (g *GlobalInventoryModal) build() {
	cfg := config.Global()
	mx := (cfg.ScreenWidth - giModalW) / 2
	my := (cfg.ScreenHeight - giModalH) / 2
	if my < 10 {
		my = 10
	}

	g.modal = minui.NewModal("gi_modal", "Global Inventory", giModalW, giModalH)
	g.modal.SetPosition(mx, my)
	g.modal.Closeable = false

	c := g.wm.Campaign
	tabX := giSideX
	addTab := func(id, label string) {
		text := label
		if id == g.activeTab {
			text = "▸ " + label
		}
		btn := minui.NewButton("gi_tab_"+id, text)
		btn.SetPosition(tabX, giTabRowY)
		btn.SetSize(giTabBtnW, giTabBtnH)
		if id == g.activeTab {
			// Disable the active tab's button so it visually reads "you're here".
			btn.SetEnabled(false)
		} else {
			btn.OnClick = func() {
				g.activeTab = id
				g.build()
			}
		}
		g.modal.AddChild(btn)
		tabX += giTabBtnW + giTabGap
	}
	addTab("current", "Current Site")
	if c.HasTech(techGlobalInvAll) {
		addTab("all", "All Sites")
	}
	if c.HasTech(techGlobalInvDetailed) {
		addTab("site", "By Site")
	}

	switch g.activeTab {
	case "current":
		g.buildCurrentTab()
	case "all":
		g.buildAllTab()
	case "site":
		g.buildSiteTab()
	}

	closeBtn := minui.NewButton("gi_close", "Close")
	closeBtn.SetPosition((giModalW-giCloseBtnW)/2, giCloseY)
	closeBtn.SetSize(giCloseBtnW, giCloseBtnH)
	closeBtn.OnClick = func() { g.Visible = false }
	g.modal.AddChild(closeBtn)
}

func (g *GlobalInventoryModal) buildCurrentTab() {
	if g.wm.current == nil || g.wm.current.level == nil {
		g.addBodyLabel("(no site loaded — Travel and Resume / Land first.)")
		return
	}
	colony := campaignColonyName(g.wm.Campaign)
	summary := storage.Summarize(
		storage.LevelProvider{Level: g.wm.current.level},
		[]string{colony},
	)
	g.addMaterialList("gi_current_mats", summary, giSideX, giBodyY, giModalW-2*giSideX, giBodyH)
}

func (g *GlobalInventoryModal) buildAllTab() {
	summary := g.aggregateAllSummaries()
	g.addMaterialList("gi_all_mats", summary, giSideX, giBodyY, giModalW-2*giSideX, giBodyH)
}

func (g *GlobalInventoryModal) buildSiteTab() {
	sites := g.collectSites()
	if g.selectedSiteKey == "" && len(sites) > 0 {
		g.selectedSiteKey = sites[0].key
	}

	items := make([]string, 0, len(sites))
	g.siteKeys = g.siteKeys[:0]
	for _, s := range sites {
		items = append(items, s.label)
		g.siteKeys = append(g.siteKeys, s.key)
	}
	if len(items) == 0 {
		items = []string{"(no established sites)"}
	}
	siteList := minui.NewListBox("gi_sites", nil)
	siteList.SetBounds(minui.Rect{X: giSideX, Y: giBodyY, Width: giSiteListW, Height: giBodyH})
	siteList.Layout() // compute visibleItems; otherwise only one row renders
	siteList.SetItems(items)
	siteList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(g.siteKeys) {
			return
		}
		g.selectedSiteKey = g.siteKeys[idx]
		g.build()
	}
	for i, k := range g.siteKeys {
		if k == g.selectedSiteKey {
			siteList.SelectedIndex = i
			break
		}
	}
	g.modal.AddChild(siteList)

	summary := g.siteSummary(g.selectedSiteKey)
	g.addMaterialList("gi_site_mats", summary, giMaterialX, giBodyY, giMaterialW, giBodyH)
}

// addMaterialList renders summary as a ListBox. Empty summaries get a plain
// "(no materials)" label instead of an empty listbox.
func (g *GlobalInventoryModal) addMaterialList(id string, summary map[string]int, x, y, w, h int) {
	if len(summary) == 0 {
		lbl := minui.NewLabel(id+"_empty", "(no materials)")
		lbl.SetPosition(x+8, y+8)
		g.modal.AddChild(lbl)
		return
	}
	blueprints := make([]string, 0, len(summary))
	for bp := range summary {
		blueprints = append(blueprints, bp)
	}
	sort.Strings(blueprints)

	items := make([]string, 0, len(blueprints))
	for _, bp := range blueprints {
		items = append(items, fmt.Sprintf("%-22s × %d", displayMaterialName(bp), summary[bp]))
	}
	list := minui.NewListBox(id, nil)
	list.SetBounds(minui.Rect{X: x, Y: y, Width: w, Height: h})
	list.Layout() // compute visibleItems; otherwise only one row renders
	list.SetItems(items)
	g.modal.AddChild(list)
}

func (g *GlobalInventoryModal) addBodyLabel(text string) {
	lbl := minui.NewLabel("gi_body_lbl", text)
	lbl.SetPosition(giSideX+8, giBodyY+8)
	g.modal.AddChild(lbl)
}

// aggregateAllSummaries totals ship hold + current site (live) + every parked
// location's cached summary.
func (g *GlobalInventoryModal) aggregateAllSummaries() map[string]int {
	out := make(map[string]int)
	if g.wm.Campaign == nil {
		return out
	}
	for bp, qty := range storage.Summarize(
		storage.ShipProvider{Level: g.wm.ShipLevel()},
		[]string{campaign.ShipSettlementName},
	) {
		out[bp] += qty
	}
	if g.wm.current != nil && g.wm.current.level != nil {
		colony := campaignColonyName(g.wm.Campaign)
		for bp, qty := range storage.Summarize(
			storage.LevelProvider{Level: g.wm.current.level},
			[]string{colony},
		) {
			out[bp] += qty
		}
	}
	currentID := g.wm.Campaign.CurrentLocationID
	for id, loc := range g.wm.Campaign.Locations {
		if id == currentID || loc.SaveFile == "" {
			continue
		}
		for bp, qty := range loc.StorageSummary {
			out[bp] += qty
		}
	}
	return out
}

type giSiteEntry struct {
	key   string // "ship" or location ID
	label string
}

// collectSites returns Ship Hold first, then every established location
// (anything with a SaveFile). Unvisited / undiscovered locations are skipped,
// but the currently-loaded site is included even before its first Freeze —
// its level is in memory, so we can still report live counts for it.
func (g *GlobalInventoryModal) collectSites() []giSiteEntry {
	out := []giSiteEntry{{key: "ship", label: "Ship Hold"}}
	if g.wm.Campaign == nil {
		return out
	}
	currentID := g.wm.Campaign.CurrentLocationID
	hasLoadedCurrent := g.wm.current != nil && g.wm.current.level != nil
	var ids []string
	for id, loc := range g.wm.Campaign.Locations {
		established := loc.SaveFile != ""
		isLoadedCurrent := id == currentID && hasLoadedCurrent
		if !established && !isLoadedCurrent {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return g.wm.Campaign.Locations[ids[i]].Name < g.wm.Campaign.Locations[ids[j]].Name
	})
	for _, id := range ids {
		loc := g.wm.Campaign.Locations[id]
		label := loc.Name
		if id == g.wm.Campaign.CurrentLocationID {
			label += "  (in orbit)"
		}
		out = append(out, giSiteEntry{key: id, label: label})
	}
	return out
}

// siteSummary returns the storage totals for one site key. Ship hold and the
// currently-loaded location compute live; every other site reads its cached
// StorageSummary (refreshed on Freeze).
func (g *GlobalInventoryModal) siteSummary(key string) map[string]int {
	if g.wm.Campaign == nil {
		return nil
	}
	if key == "ship" {
		return storage.Summarize(
			storage.ShipProvider{Level: g.wm.ShipLevel()},
			[]string{campaign.ShipSettlementName},
		)
	}
	loc := g.wm.Campaign.Locations[key]
	if loc == nil {
		return nil
	}
	if key == g.wm.Campaign.CurrentLocationID && g.wm.current != nil && g.wm.current.level != nil {
		return storage.Summarize(
			storage.LevelProvider{Level: g.wm.current.level},
			[]string{campaignColonyName(g.wm.Campaign)},
		)
	}
	return loc.StorageSummary
}

func (g *GlobalInventoryModal) Update() {
	if !g.Visible {
		return
	}
	g.modal.Update()
}

func (g *GlobalInventoryModal) Draw(screen *ebiten.Image) {
	if !g.Visible {
		return
	}
	g.modal.Draw(screen)
}
