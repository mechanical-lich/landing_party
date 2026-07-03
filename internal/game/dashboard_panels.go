package game

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

var (
	dashTitleColor  = color.RGBA{120, 200, 255, 255}
	dashBodyColor   = color.RGBA{180, 200, 220, 255}
	dashLockedColor = color.RGBA{200, 150, 110, 255}
)

// ---- Quests ------------------------------------------------------------------

type questsPanel struct {
	campaign *campaign.Campaign
	wm       *WorldManager
	list     *minui.ListBox
	accept   *minui.Button
	quests   []*campaign.Quest
	status   string
}

func newQuestsPanel(c *campaign.Campaign, wm *WorldManager) *questsPanel {
	p := &questsPanel{campaign: c, wm: wm}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	p.list = minui.NewListBox("dash_ql_list", nil)
	p.list.SetBounds(minui.Rect{X: cx - 300, Y: dashContentTop + 16, Width: 600, Height: 300})
	p.list.Layout()
	p.accept = minui.NewButton("dash_ql_accept", "Accept Quest")
	p.accept.SetPosition(cx-300, dashContentTop+420)
	p.accept.SetSize(200, 36)
	p.accept.OnClick = func() { p.acceptSelected() }
	p.refresh()
	return p
}

func (p *questsPanel) Enter() { p.refresh() }

func (p *questsPanel) refresh() {
	p.quests = p.quests[:0]
	var labels []string
	add := func(qs []*campaign.Quest, tag string) {
		for _, def := range qs {
			p.quests = append(p.quests, def)
			labels = append(labels, fmt.Sprintf("[%s] %s", tag, def.Name))
		}
	}
	add(p.campaign.AvailableQuests(), "OFFER")
	add(p.campaign.ActiveQuests(), "ACTIVE")
	add(p.campaign.CompletedQuests(), "DONE")
	if len(labels) == 0 {
		labels = []string{"(no quests)"}
	}
	sel := p.list.SelectedIndex
	p.list.SetItems(labels)
	if sel >= 0 && sel < len(p.quests) {
		p.list.SelectedIndex = sel
	} else if len(p.quests) > 0 {
		p.list.SelectedIndex = 0
	}
}

func (p *questsPanel) selected() *campaign.Quest {
	i := p.list.SelectedIndex
	if i < 0 || i >= len(p.quests) {
		return nil
	}
	return p.quests[i]
}

func (p *questsPanel) acceptSelected() {
	def := p.selected()
	if def == nil {
		return
	}
	if p.campaign.AcceptQuest(def.ID) {
		p.status = "Accepted: " + def.Name
		if def.Location != "" {
			name := def.Location
			if loc := p.campaign.Locations[def.Location]; loc != nil {
				name = loc.Name
			}
			p.status += " — " + name + " revealed on the Star Map"
		}
		p.refresh()
	} else {
		p.status = "That quest can't be accepted."
	}
}

func (p *questsPanel) Update() {
	p.list.Update()
	p.accept.Update()
}

func (p *questsPanel) Draw(screen *ebiten.Image) {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	p.list.Draw(screen)
	if def := p.selected(); def != nil {
		y := dashContentTop + 330
		mlge_text.Draw(screen, def.Name, 18, cx-300, y, color.RGBA{220, 230, 255, 255})
		mlge_text.Draw(screen, def.Description, 13, cx-300, y+26, dashBodyColor)
		if def.Location != "" {
			locName := def.Location
			if loc := p.campaign.Locations[def.Location]; loc != nil {
				locName = loc.Name
			}
			mlge_text.Draw(screen, "Location: "+locName, 13, cx-300, y+48, color.RGBA{170, 200, 240, 255})
		}
		if reward := questRewardText(def); reward != "" {
			mlge_text.Draw(screen, "Reward: "+reward, 13, cx-300, y+70, color.RGBA{150, 210, 160, 255})
		}
	}
	p.accept.Draw(screen)
	if p.status != "" {
		mlge_text.Draw(screen, p.status, 13, cx-300, dashContentTop+470, dashLockedColor)
	}
}

func (p *questsPanel) Transition() state.StateInterface { return nil }

// ---- Star Map (bridge to the classic screen until step 2) --------------------

type starMapPanel struct {
	campaign *campaign.Campaign
	wm       *WorldManager
	openBtn  *minui.Button
	pending  state.StateInterface
}

func newStarMapPanel(c *campaign.Campaign, wm *WorldManager) *starMapPanel {
	p := &starMapPanel{campaign: c, wm: wm}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	p.openBtn = minui.NewButton("dash_open_starmap", "Open Star Map")
	p.openBtn.SetPosition(cx-110, dashContentTop+120)
	p.openBtn.SetSize(220, 46)
	p.openBtn.OnClick = func() { p.pending = NewOverworldState(p.campaign, p.wm) }
	return p
}

func (p *starMapPanel) Enter()  {}
func (p *starMapPanel) Update() { p.openBtn.Update() }
func (p *starMapPanel) Draw(s *ebiten.Image) {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	mlge_text.Draw(s, "Star Map", 22, cx-60, dashContentTop+40, dashTitleColor)
	mlge_text.Draw(s, "The visual star map is coming here. For now, open the classic view to travel and beam crew.", 13, cx-330, dashContentTop+80, dashBodyColor)
	p.openBtn.Draw(s)
}
func (p *starMapPanel) Transition() state.StateInterface {
	t := p.pending
	p.pending = nil
	return t
}

// ---- Crew (placeholder until step 3) -----------------------------------------

type crewPanel struct {
	campaign *campaign.Campaign
	wm       *WorldManager
}

func newCrewPanel(c *campaign.Campaign, wm *WorldManager) *crewPanel {
	return &crewPanel{campaign: c, wm: wm}
}

func (p *crewPanel) Enter()  {}
func (p *crewPanel) Update() {}
func (p *crewPanel) Draw(s *ebiten.Image) {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	mlge_text.Draw(s, "Crew", 22, cx-40, dashContentTop+40, dashTitleColor)
	mlge_text.Draw(s, "Crew roster and per-colonist management coming soon.", 13, cx-220, dashContentTop+80, dashBodyColor)
}
func (p *crewPanel) Transition() state.StateInterface { return nil }

// ---- Global Inventory (inline) -----------------------------------------------

// globalInvPanel renders the cross-campaign storage view inline. It reuses the
// GlobalInventoryModal's data methods (aggregateAllSummaries / collectSites /
// siteSummary — the summarization logic) via a data-only instance that is never
// shown as a modal; the tab layout is rendered here.
type globalInvPanel struct {
	campaign        *campaign.Campaign
	wm              *WorldManager
	data            *GlobalInventoryModal
	activeTab       string // "current" | "all" | "site"
	selectedSiteKey string
	siteKeys        []string
	subTabs         []*minui.Button
	body            []minui.Element
	dirty           bool
}

func newGlobalInvPanel(c *campaign.Campaign, wm *WorldManager) *globalInvPanel {
	return &globalInvPanel{campaign: c, wm: wm, data: newGlobalInventoryModal(wm)}
}

func (p *globalInvPanel) Enter() {
	// Default to the highest tier the player has unlocked.
	switch {
	case p.campaign.HasTech(techGlobalInvDetailed):
		p.activeTab = "site"
	case p.campaign.HasTech(techGlobalInvAll):
		p.activeTab = "all"
	default:
		p.activeTab = "current"
	}
	p.selectedSiteKey = ""
	p.dirty = true
}

func (p *globalInvPanel) rebuild() {
	p.dirty = false
	p.subTabs = nil
	p.body = nil
	cfg := config.Global()

	x := 20
	addTab := func(id, label string) {
		text := label
		if id == p.activeTab {
			text = "▸ " + label
		}
		b := minui.NewButton("dash_gi_tab_"+id, text)
		b.SetPosition(x, dashContentTop+6)
		b.SetSize(150, 30)
		if id == p.activeTab {
			b.SetEnabled(false)
		} else {
			id := id
			b.OnClick = func() { p.activeTab = id; p.dirty = true }
		}
		p.subTabs = append(p.subTabs, b)
		x += 156
	}
	addTab("current", "Current Site")
	if p.campaign.HasTech(techGlobalInvAll) {
		addTab("all", "All Sites")
	}
	if p.campaign.HasTech(techGlobalInvDetailed) {
		addTab("site", "By Site")
	}

	bodyX, bodyY := 20, dashContentTop+46
	fullW := cfg.ScreenWidth - 40
	bodyH := cfg.ScreenHeight - bodyY - 40

	switch p.activeTab {
	case "current":
		var summary map[string]int
		if p.wm.current != nil && p.wm.current.level != nil {
			summary = storage.Summarize(
				storage.LevelProvider{Level: p.wm.current.level},
				[]string{campaignColonyName(p.wm.Campaign)},
			)
		}
		p.addMaterialBody("dash_gi_cur", summary, bodyX, bodyY, fullW, bodyH, "(no site loaded — travel and land first.)")
	case "all":
		p.addMaterialBody("dash_gi_all", p.data.aggregateAllSummaries(), bodyX, bodyY, fullW, bodyH, "(no materials)")
	case "site":
		sites := p.data.collectSites()
		if p.selectedSiteKey == "" && len(sites) > 0 {
			p.selectedSiteKey = sites[0].key
		}
		items := make([]string, 0, len(sites))
		p.siteKeys = p.siteKeys[:0]
		for _, s := range sites {
			items = append(items, s.label)
			p.siteKeys = append(p.siteKeys, s.key)
		}
		if len(items) == 0 {
			items = []string{"(no established sites)"}
		}
		siteList := minui.NewListBox("dash_gi_sites", nil)
		siteList.SetBounds(minui.Rect{X: bodyX, Y: bodyY, Width: 220, Height: bodyH})
		siteList.Layout()
		siteList.SetItems(items)
		siteList.OnSelect = func(idx int, _ string) {
			if idx < 0 || idx >= len(p.siteKeys) {
				return
			}
			p.selectedSiteKey = p.siteKeys[idx]
			p.dirty = true
		}
		for i, k := range p.siteKeys {
			if k == p.selectedSiteKey {
				siteList.SelectedIndex = i
				break
			}
		}
		p.body = append(p.body, siteList)

		matX := bodyX + 232
		p.addMaterialBody("dash_gi_site", p.data.siteSummary(p.selectedSiteKey), matX, bodyY, cfg.ScreenWidth-matX-20, bodyH, "(no materials)")
	}
}

func (p *globalInvPanel) addMaterialBody(id string, summary map[string]int, x, y, w, h int, emptyMsg string) {
	if len(summary) == 0 {
		lbl := minui.NewLabel(id+"_empty", emptyMsg)
		lbl.SetPosition(x+8, y+8)
		p.body = append(p.body, lbl)
		return
	}
	bps := make([]string, 0, len(summary))
	for bp := range summary {
		bps = append(bps, bp)
	}
	sort.Strings(bps)
	items := make([]string, 0, len(bps))
	for _, bp := range bps {
		items = append(items, fmt.Sprintf("%-22s × %d", displayMaterialName(bp), summary[bp]))
	}
	list := minui.NewListBox(id, nil)
	list.SetBounds(minui.Rect{X: x, Y: y, Width: w, Height: h})
	list.Layout()
	list.SetItems(items)
	p.body = append(p.body, list)
}

func (p *globalInvPanel) Update() {
	if !p.campaign.HasTech(techGlobalInvCurrent) {
		return
	}
	if p.dirty || p.subTabs == nil {
		p.rebuild()
	}
	for _, b := range p.subTabs {
		b.Update()
	}
	for _, e := range p.body {
		e.Update()
	}
}

func (p *globalInvPanel) Draw(s *ebiten.Image) {
	cx := config.Global().ScreenWidth / 2
	if !p.campaign.HasTech(techGlobalInvCurrent) {
		mlge_text.Draw(s, "Global Inventory", 22, cx-120, dashContentTop+40, dashTitleColor)
		mlge_text.Draw(s, "Requires Inventory Survey research.", 13, cx-160, dashContentTop+80, dashLockedColor)
		return
	}
	for _, b := range p.subTabs {
		b.Draw(s)
	}
	for _, e := range p.body {
		e.Draw(s)
	}
}

func (p *globalInvPanel) Transition() state.StateInterface { return nil }

// ---- Encyclopedia (inline) ---------------------------------------------------

type encyclopediaPanel struct {
	campaign   *campaign.Campaign
	wm         *WorldManager
	list       *minui.ListBox
	blueprints []string
	descCache  map[string]*rlcomponents.DescriptionComponent
}

func newEncyclopediaPanel(c *campaign.Campaign, wm *WorldManager) *encyclopediaPanel {
	p := &encyclopediaPanel{
		campaign:  c,
		wm:        wm,
		descCache: map[string]*rlcomponents.DescriptionComponent{},
	}
	cfg := config.Global()
	p.list = minui.NewListBox("dash_ency_list", nil)
	listY := dashContentTop + 78
	p.list.SetBounds(minui.Rect{X: 40, Y: listY, Width: 320, Height: cfg.ScreenHeight - listY - 40})
	p.list.Layout()
	p.populate()
	return p
}

func (p *encyclopediaPanel) Enter() { p.populate() }

func (p *encyclopediaPanel) populate() {
	type entry struct{ bp, name string }
	var entries []entry
	for _, bp := range p.campaign.KnownEntities {
		entries = append(entries, entry{bp: bp, name: displayName(bp, p.lookupDescription(bp))})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})
	items := make([]string, 0, len(entries))
	bps := make([]string, 0, len(entries))
	for _, en := range entries {
		items = append(items, en.name)
		bps = append(bps, en.bp)
	}
	if len(items) == 0 {
		items = []string{"(no entries yet — hover entities to register them)"}
		bps = []string{""}
	}
	p.blueprints = bps
	p.list.SetItems(items)
	if len(bps) > 0 && bps[0] != "" && p.list.SelectedIndex < 0 {
		p.list.SelectedIndex = 0
	}
}

func (p *encyclopediaPanel) lookupDescription(bp string) *rlcomponents.DescriptionComponent {
	if bp == "" {
		return nil
	}
	if d, ok := p.descCache[bp]; ok {
		return d
	}
	d := factory.GetDescription(bp)
	p.descCache[bp] = d
	return d
}

func (p *encyclopediaPanel) selectedBlueprint() string {
	i := p.list.SelectedIndex
	if i < 0 || i >= len(p.blueprints) {
		return ""
	}
	return p.blueprints[i]
}

func (p *encyclopediaPanel) Update() { p.list.Update() }

func (p *encyclopediaPanel) Draw(screen *ebiten.Image) {
	cfg := config.Global()
	hdr := "Discovered: " + itoa(len(p.campaign.KnownEntities))
	mlge_text.Draw(screen, hdr, 14, 40, dashContentTop+20, color.RGBA{160, 185, 215, 255})
	mlge_text.Draw(screen, "Catalogue", 16, 40, dashContentTop+50, color.RGBA{200, 220, 255, 255})
	p.list.Draw(screen)

	detailX := 400
	detailY := dashContentTop + 24
	detailW := cfg.ScreenWidth - detailX - 40

	bp := p.selectedBlueprint()
	if bp == "" {
		mlge_text.Draw(screen, "Select an entry from the catalogue.", 14, detailX, detailY+12, color.RGBA{160, 185, 215, 255})
		return
	}
	desc := p.lookupDescription(bp)

	const portraitPx = 72
	if ac := factory.GetAppearance(bp); ac != nil && ac.Resource != "" {
		size := ac.SpriteSize
		if size <= 0 {
			size = 24
		}
		scale := float64(portraitPx) / float64(size)
		icon := minui.NewIconWithScale(ac.Resource, ac.SpriteX, ac.SpriteY, size, size, scale)
		if ac.R != 0 || ac.G != 0 || ac.B != 0 {
			icon.Tint = color.RGBA{R: ac.R, G: ac.G, B: ac.B, A: 255}
		}
		icon.Draw(screen, detailX, detailY-4)
	}

	titleX := detailX + portraitPx + 16
	titleY := detailY + 10
	mlge_text.Draw(screen, displayName(bp, desc), 24, titleX, titleY, color.RGBA{230, 240, 255, 255})

	subY := titleY + 30
	if desc != nil && desc.Faction != "" {
		mlge_text.Draw(screen, "Faction: "+desc.Faction, 13, titleX, subY, color.RGBA{170, 200, 240, 255})
		subY += 18
	}
	if desc != nil && len(desc.Tags) > 0 {
		mlge_text.Draw(screen, "Tags: "+strings.Join(desc.Tags, ", "), 13, titleX, subY, color.RGBA{170, 200, 240, 255})
	}

	bodyY := detailY + 110
	bodyText := "No description recorded."
	if desc != nil && desc.LongDescription != "" {
		bodyText = desc.LongDescription
	}
	drawWrapped(screen, bodyText, 14, detailX, bodyY, detailW, 22, color.RGBA{200, 220, 240, 255})

	mlge_text.Draw(screen, "id: "+bp, 11, detailX, cfg.ScreenHeight-40, color.RGBA{90, 110, 130, 200})
}

func (p *encyclopediaPanel) Transition() state.StateInterface { return nil }
