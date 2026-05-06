package gui

import (
	"fmt"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	minui "github.com/mechanical-lich/mlge/ui/minui"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/construction"
)

// HUDScreen is the main in-game HUD: sidebar, messages, resource bar, entity detail.
type HUDScreen struct {
	uiGUI *minui.GUI
	op    *ebiten.DrawImageOptions

	// Sidebar
	sidebarTabPanel    *minui.TabPanel
	buildMenuItems     map[string]*minui.MenuItem
	buildCategoryPanel *minui.Panel
	buildSubPanel      *minui.Panel

	// Population tab
	populationVBox   *minui.VBox
	populationPanel  *minui.Panel
	populationLabels []*minui.Label
	lastPopEntries   []PopulationEntry

	// Goals tab
	goalsVBox     *minui.VBox
	goalsPanel    *minui.Panel
	goalsLabels   []*minui.Label
	lastGoalLines []string

	// HUD elements
	messagesTextArea *minui.ScrollingTextArea
	resourceBar      *minui.ResourceBar

	// Main menu modal
	mainMenuModal *minui.Modal

	// Entity detail panel
	detailsPanel   *minui.Panel
	detailsVBox    *minui.VBox
	hoveredEntity  *ecs.Entity
	selectedEntity *ecs.Entity

	// Cursor
	CursorImage *ebiten.Image
}

func NewHUDScreen() *HUDScreen {
	theme := minui.NewDarkTheme()
	h := &HUDScreen{
		uiGUI: minui.NewGUIWithTheme(theme),
		op:    &ebiten.DrawImageOptions{},
	}
	h.setupHUDElements()
	h.setupSidebar()
	h.setupModals()
	h.registerListeners()
	return h
}

func (h *HUDScreen) OnEnter()       {}
func (h *HUDScreen) OnExit()        {}
func (h *HUDScreen) IsOpaque() bool { return true }

func (h *HUDScreen) Update() {
	h.uiGUI.Update()
	h.uiGUI.Layout()
}

func (h *HUDScreen) Draw(screen *ebiten.Image) {
	h.uiGUI.Draw(screen)
	h.drawCursor(screen)
}

func (h *HUDScreen) drawCursor(screen *ebiten.Image) {
	if h.CursorImage != nil {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
		cX, cY := ebiten.CursorPosition()
		h.op.GeoM.Reset()
		h.op.GeoM.Translate(float64(cX), float64(cY))
		screen.DrawImage(h.CursorImage, h.op)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
}

func (h *HUDScreen) setupHUDElements() {
	cfg := config.Global()
	sw, sh := cfg.ScreenWidth, cfg.ScreenHeight

	const msgW, msgH = 400, 120
	h.messagesTextArea = minui.NewScrollingTextArea("messagesLog", msgW, msgH)
	h.messagesTextArea.SetPosition(sw-msgW-16, sh-msgH-10)
	h.uiGUI.AddElement(h.messagesTextArea)

	h.resourceBar = minui.NewResourceBar("resourceBar")
	h.resourceBar.SetBounds(minui.Rect{X: 210, Y: sh - 35, Width: 400, Height: 30})
	h.resourceBar.AddResource("metal_ore", nil, 0)
	h.resourceBar.AddResource("crystal", nil, 0)
	h.uiGUI.AddElement(h.resourceBar)
}

func (h *HUDScreen) setupSidebar() {
	sh := config.Global().ScreenHeight

	h.sidebarTabPanel = minui.NewTabPanel("sidebar", 200, sh)
	h.sidebarTabPanel.SetPosition(0, 0)
	h.sidebarTabPanel.TabPosition = minui.TabsTop
	h.sidebarTabPanel.TabHeight = 28

	buildContent := minui.NewPanel("buildTabContent")
	h.setupBuildTab(buildContent)
	h.sidebarTabPanel.AddTab("build", "Build", buildContent)

	h.populationPanel = minui.NewPanel("populationTabContent")
	h.populationVBox = minui.NewVBox("populationVBox")
	h.populationVBox.SetPosition(6, 6)
	h.populationVBox.Spacing = 3
	h.populationPanel.AddChild(h.populationVBox)
	h.sidebarTabPanel.AddTab("population", "Pop", h.populationPanel)

	h.goalsPanel = minui.NewPanel("goalsTabContent")
	h.goalsVBox = minui.NewVBox("goalsVBox")
	h.goalsVBox.SetPosition(6, 6)
	h.goalsVBox.Spacing = 3
	h.goalsPanel.AddChild(h.goalsVBox)
	h.sidebarTabPanel.AddTab("goals", "Goals", h.goalsPanel)

	h.uiGUI.AddElement(h.sidebarTabPanel)
}

type hudBuildOrderItem struct {
	id    string
	label string
	mode  CursorModeType
}

func (h *HUDScreen) setupBuildTab(panel *minui.Panel) {
	h.buildMenuItems = make(map[string]*minui.MenuItem)

	const itemH = 24
	const panelW = 192

	type buildCategory struct {
		id    string
		label string
		items []hudBuildOrderItem
		types []string
	}

	categories := []buildCategory{
		{
			id:    "orders",
			label: "Orders",
			items: []hudBuildOrderItem{
				{"default", "Default", CursorModeDefault},
				{"dig", "Dig", CursorModeDig},
				{"mine", "Mine", CursorModeMine},
				{"cancel", "Cancel Task", CursorModeCancel},
				{"attack", "Attack", CursorModeAttack},
			},
		},
		{id: "walls", label: "Walls", types: []string{"hull_wall"}},
		{id: "floors", label: "Floors", types: []string{"hull_floor"}},
		{id: "stairs", label: "Stairs", types: []string{"stairs_up", "stairs_down"}},
		{id: "structures", label: "Structures", types: []string{"storage_locker", "research_lab"}},
	}

	h.buildCategoryPanel = minui.NewPanel("buildCategories")
	h.buildCategoryPanel.SetBounds(minui.Rect{X: 0, Y: 0, Width: panelW, Height: 600})

	h.buildSubPanel = minui.NewPanel("buildSubMenu")
	h.buildSubPanel.SetBounds(minui.Rect{X: 0, Y: 0, Width: panelW, Height: 600})
	h.buildSubPanel.SetVisible(false)

	showSubMenu := func(title string, items []hudBuildOrderItem, types []string) {
		snapshot := make([]minui.Element, len(h.buildSubPanel.GetChildren()))
		copy(snapshot, h.buildSubPanel.GetChildren())
		for _, child := range snapshot {
			h.buildSubPanel.RemoveChild(child)
		}

		y := 4

		back := minui.NewMenuItem("sub_back", "← Back")
		back.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
		back.OnClick = func() {
			h.buildSubPanel.SetVisible(false)
			h.buildCategoryPanel.SetVisible(true)
		}
		h.buildSubPanel.AddChild(back)
		y += itemH

		hdr := minui.NewMenuHeader("sub_hdr", title)
		hdr.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: 18})
		h.buildSubPanel.AddChild(hdr)
		y += 20

		for _, oi := range items {
			oiID := oi.id
			oiMode := oi.mode
			mi := minui.NewMenuItem("order_"+oi.id, oi.label)
			mi.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
			mi.OnClick = func() {
				h.selectBuildItem(oiID)
				event.GetQueuedInstance().QueueEvent(CursorModeChangedEvent{Mode: oiMode})
			}
			h.buildMenuItems[oiID] = mi
			h.buildSubPanel.AddChild(mi)
			y += itemH
		}

		for _, buildType := range types {
			buildable := construction.GetBuildable(buildType)
			if buildable.Name == "" || buildable.Hidden {
				continue
			}
			btID := buildType
			mi := minui.NewMenuItem("build_"+buildType, buildable.Name)
			mi.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
			mi.OnClick = func() {
				h.selectBuildItem("build_" + btID)
				event.GetQueuedInstance().QueueEvent(BuildOptionChangedEvent{Option: btID})
				event.GetQueuedInstance().QueueEvent(CursorModeChangedEvent{Mode: CursorModeBuild})
			}
			h.buildMenuItems["build_"+buildType] = mi
			h.buildSubPanel.AddChild(mi)
			y += itemH
		}

		h.buildCategoryPanel.SetVisible(false)
		h.buildSubPanel.SetVisible(true)
	}

	y := 4
	for _, cat := range categories {
		catCopy := cat
		mi := minui.NewMenuItem("cat_"+cat.id, cat.label+" >")
		mi.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
		mi.OnClick = func() {
			showSubMenu(catCopy.label, catCopy.items, catCopy.types)
		}
		h.buildCategoryPanel.AddChild(mi)
		y += itemH
	}

	panel.AddChild(h.buildCategoryPanel)
	panel.AddChild(h.buildSubPanel)
}

func (h *HUDScreen) selectBuildItem(selectedID string) {
	for id, item := range h.buildMenuItems {
		item.SetSelected(id == selectedID)
	}
}

func (h *HUDScreen) setupModals() {
	cfg := config.Global()
	sw, sh := cfg.ScreenWidth, cfg.ScreenHeight

	h.detailsPanel = minui.NewPanel("details")
	h.detailsPanel.SetPosition(sw-240, sh-10)
	h.detailsPanel.SetSize(220, 10)
	h.detailsPanel.SetVisible(false)
	h.uiGUI.AddElement(h.detailsPanel)

	h.detailsVBox = minui.NewVBox("detailsContent")
	h.detailsVBox.SetPosition(8, 8)
	h.detailsVBox.Spacing = 3
	h.detailsPanel.AddChild(h.detailsVBox)

	h.mainMenuModal = minui.NewModal("mainMenu", "Main Menu", 300, 280)
	h.mainMenuModal.SetPosition(sw/2-150, sh/2-140)
	h.mainMenuModal.SetVisible(false)
	h.mainMenuModal.Closeable = true

	menuVBox := minui.NewVBox("mainMenuButtons")
	menuVBox.SetPosition(30, 50)
	menuVBox.Spacing = 10
	for _, entry := range []struct{ id, label string }{
		{"newgame", "New Game"},
		{"quit", "Quit"},
	} {
		action := entry.id
		btn := minui.NewButton(entry.id, entry.label)
		btn.OnClick = func() {
			event.GetQueuedInstance().QueueEvent(MainMenuEvent{Action: action})
		}
		menuVBox.AddChild(btn)
	}
	h.mainMenuModal.AddChild(menuVBox)
	h.uiGUI.AddModal(h.mainMenuModal)
}

func (h *HUDScreen) registerListeners() {
	event.GetQueuedInstance().RegisterListener(h, message.MessageEventType)
	event.GetQueuedInstance().RegisterListener(h, EntitySelectedEventType)
	event.GetQueuedInstance().RegisterListener(h, minui.EventTypeModalClose)
}

func (h *HUDScreen) HandleEvent(evt event.EventData) error {
	switch e := evt.(type) {
	case message.MessageEvent:
		h.messagesTextArea.AddText(fmt.Sprintf("%s: %s", e.Sender, e.Message))
	case EntitySelectedEvent:
		_ = e
	case minui.ModalCloseEvent:
		_ = e
	}
	return nil
}

// ---- Public API ------------------------------------------------------------

func (h *HUDScreen) ActiveSidebarTab() string {
	if h.sidebarTabPanel == nil {
		return ""
	}
	return h.sidebarTabPanel.ActiveTabID
}

func (h *HUDScreen) RefreshPopulationTab(entries []PopulationEntry) {
	if popEntriesEqual(h.lastPopEntries, entries) {
		return
	}
	h.lastPopEntries = entries

	fontSize := 13
	want := len(entries) + 1

	for len(h.populationLabels) < want {
		lbl := minui.NewLabel(fmt.Sprintf("pop_%d", len(h.populationLabels)), "")
		lbl.GetStyle().FontSize = &fontSize
		h.populationLabels = append(h.populationLabels, lbl)
		h.populationVBox.AddChild(lbl)
	}
	for len(h.populationLabels) > want {
		last := h.populationLabels[len(h.populationLabels)-1]
		h.populationVBox.RemoveChild(last)
		h.populationLabels = h.populationLabels[:len(h.populationLabels)-1]
	}

	h.populationLabels[0].Text = fmt.Sprintf("Colonists: %d", len(entries))
	for i, entry := range entries {
		h.populationLabels[i+1].Text = fmt.Sprintf("%s  [%s/%s]", entry.Name, entry.State, entry.Task)
	}
}

func (h *HUDScreen) RefreshGoalsTab(lines []string) {
	if goalLinesEqual(h.lastGoalLines, lines) {
		return
	}
	h.lastGoalLines = lines

	fontSize := 13
	for len(h.goalsLabels) < len(lines) {
		lbl := minui.NewLabel(fmt.Sprintf("goal_%d", len(h.goalsLabels)), "")
		lbl.GetStyle().FontSize = &fontSize
		h.goalsLabels = append(h.goalsLabels, lbl)
		h.goalsVBox.AddChild(lbl)
	}
	for len(h.goalsLabels) > len(lines) {
		last := h.goalsLabels[len(h.goalsLabels)-1]
		h.goalsVBox.RemoveChild(last)
		h.goalsLabels = h.goalsLabels[:len(h.goalsLabels)-1]
	}

	for i, line := range lines {
		h.goalsLabels[i].Text = line
	}
}

func (h *HUDScreen) SetHoveredEntity(entity *ecs.Entity) {
	if entity == h.hoveredEntity {
		return
	}
	h.hoveredEntity = entity
	h.selectedEntity = entity
	if entity == nil {
		h.detailsPanel.SetVisible(false)
		return
	}
	h.updateDetailsContent()
	h.detailsPanel.SetVisible(true)
}

// SetHoveredTile shows tile info in the detail panel when no entity is under the cursor.
func (h *HUDScreen) SetHoveredTile(name string, solid, water, air, space bool) {
	h.hoveredEntity = nil
	h.selectedEntity = nil

	h.detailsPanel.RemoveChild(h.detailsVBox)
	h.detailsVBox = minui.NewVBox("detailsContent")
	h.detailsVBox.Spacing = 3
	h.detailsPanel.AddChild(h.detailsVBox)

	labelID := 0
	add := func(text string) {
		h.detailsVBox.AddChild(minui.NewLabel(fmt.Sprintf("detail_%d", labelID), text))
		labelID++
	}

	add("Tile: " + name)
	flags := ""
	if solid {
		flags += " solid"
	}
	if water {
		flags += " water"
	}
	if air {
		flags += " air"
	}
	if space {
		flags += " space"
	}
	if flags != "" {
		add("Flags:" + flags)
	}

	h.resizeDetailPanel()
	h.detailsPanel.SetVisible(true)
}

func (h *HUDScreen) ClearHover() {
	if h.hoveredEntity == nil && !h.detailsPanel.IsVisible() {
		return
	}
	h.hoveredEntity = nil
	h.selectedEntity = nil
	h.detailsPanel.SetVisible(false)
}

func (h *HUDScreen) GetInputFocused() bool            { return h.uiGUI.GetKeyboardFocused() }
func (h *HUDScreen) GetMouseFocused() bool            { return h.uiGUI.GetMouseFocused() }
func (h *HUDScreen) WithinModalBounds(x, y int) bool  { return h.uiGUI.WithinModalBounds(x, y) }

func (h *HUDScreen) OpenModal(name string) {
	switch name {
	case "mainMenu":
		h.mainMenuModal.SetVisible(true)
	}
}

func (h *HUDScreen) CloseModal(name string) {
	switch name {
	case "mainMenu":
		h.mainMenuModal.SetVisible(false)
	}
}

func (h *HUDScreen) ModalOpen(name string) bool {
	switch name {
	case "mainMenu":
		return h.mainMenuModal.IsVisible()
	}
	return false
}

func (h *HUDScreen) UpdateResource(id string, value int) {
	if h.resourceBar != nil {
		h.resourceBar.SetResourceValue(id, value)
	}
}

func (h *HUDScreen) updateDetailsContent() {
	h.detailsPanel.RemoveChild(h.detailsVBox)
	h.detailsVBox = minui.NewVBox("detailsContent")
	h.detailsVBox.Spacing = 3
	h.detailsPanel.AddChild(h.detailsVBox)

	if h.selectedEntity == nil {
		return
	}

	entity := h.selectedEntity
	labelID := 0
	add := func(text string) {
		h.detailsVBox.AddChild(minui.NewLabel(fmt.Sprintf("detail_%d", labelID), text))
		labelID++
	}

	if entity.HasComponent(rlcomponents.Position) {
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		add(fmt.Sprintf("Position: (%d, %d, %d)", pc.GetX(), pc.GetY(), pc.GetZ()))
	}
	if entity.HasComponent(rlcomponents.Description) {
		dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		add("Name: " + dc.Name)
		if dc.Faction != "" {
			add("Faction: " + dc.Faction)
		}
	}
	if entity.HasComponent(rlcomponents.Health) {
		hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		add("HP: " + strconv.Itoa(hc.Health) + "/" + strconv.Itoa(hc.MaxHealth))
	}
	if entity.HasComponent(rlcomponents.Stats) {
		sc := entity.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		add(fmt.Sprintf("Str:%d Dex:%d Int:%d AC:%d", sc.Str, sc.Dex, sc.Int, sc.AC))
	}
	if entity.HasComponent(rlcomponents.AIMemory) {
		aiMem := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		add("State: " + string(aiMem.State))
	}
	if entity.HasComponent(components.Worker) {
		wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			add("Task: " + string(wc.CurrentTask.Action))
		} else {
			add("Task: idle")
		}
	}
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		if len(inv.Bag) > 0 {
			add(fmt.Sprintf("Bag (%d):", len(inv.Bag)))
			for _, item := range inv.Bag {
				if d := item.GetComponent(rlcomponents.Description); d != nil {
					add("  - " + d.(*rlcomponents.DescriptionComponent).Name)
				}
			}
		} else {
			add("Bag: empty")
		}
	}
	if entity.HasComponent(components.Storage) {
		st := entity.GetComponent(components.Storage).(*components.StorageComponent)
		if len(st.Items) > 0 {
			add(fmt.Sprintf("Storage (%d):", len(st.Items)))
			for _, item := range st.Items {
				if d := item.GetComponent(rlcomponents.Description); d != nil {
					add("  - " + d.(*rlcomponents.DescriptionComponent).Name)
				}
			}
		} else {
			add("Storage: empty")
		}
	}

	h.resizeDetailPanel()
}

func (h *HUDScreen) resizeDetailPanel() {
	const panelW = 220
	const padding = 8
	const margin = 8

	h.detailsVBox.Layout()
	panelH := h.detailsVBox.GetHeight() + padding*2

	cfg := config.Global()
	h.detailsPanel.SetSize(panelW, panelH)
	h.detailsPanel.SetPosition(cfg.ScreenWidth-panelW-margin, margin)
	h.detailsVBox.SetPosition(padding, padding)
}

func popEntriesEqual(a, b []PopulationEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func goalLinesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
