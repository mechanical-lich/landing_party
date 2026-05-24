package gui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	minui "github.com/mechanical-lich/mlge/ui/minui"
)

// scrollingVBox pairs a ScrollPanel with an inner VBox so that children
// can be added/removed like a normal VBox while mouse-wheel scrolling works.
// It overrides Update() to keep the VBox offset in sync with the scroll position.
type scrollingVBox struct {
	*minui.ScrollPanel
	vbox *minui.VBox
}

func newScrollingVBox(id string, spacing int) *scrollingVBox {
	sp := minui.NewScrollPanel(id)
	vb := minui.NewVBox(id + "_content")
	vb.Spacing = spacing
	sp.AddChild(vb)
	return &scrollingVBox{ScrollPanel: sp, vbox: vb}
}

func (s *scrollingVBox) Update() {
	s.ScrollPanel.Update()
	s.vbox.SetPosition(0, -s.ScrollPanel.ScrollOffsetY())
	s.vbox.Layout()
}

func (s *scrollingVBox) AddContent(child minui.Element)    { s.vbox.AddChild(child) }
func (s *scrollingVBox) RemoveContent(child minui.Element) { s.vbox.RemoveChild(child) }
func (s *scrollingVBox) GetContent() []minui.Element       { return s.vbox.GetChildren() }

// HUDScreen is the main in-game HUD: sidebar, messages, resource bar, entity detail.
type HUDScreen struct {
	uiGUI            *minui.GUI
	op               *ebiten.DrawImageOptions
	inputBlocked     bool
	detailsTopOffset int

	// Sidebar
	sidebarTabPanel    *minui.TabPanel
	buildMenuItems     map[string]*minui.MenuItem
	buildCategoryPanel *minui.Panel
	buildSubPanel      *minui.Panel
	knownTechs         map[string]bool
	showSubMenuFn      func(title string, items []hudBuildOrderItem, categoryID string)
	currentSubTitle    string
	currentSubItems    []hudBuildOrderItem
	currentSubCategory string

	// Population tab
	populationVBox   *minui.VBox
	populationPanel  *minui.Panel
	populationHeader *minui.Label
	rogueExitButton  *minui.Button
	populationItems  []*minui.MenuItem
	populationCtrl   []*minui.MenuItem
	lastPopEntries   []PopulationEntry

	// Colonist modal
	colonistModal         *minui.Modal
	colonistScroll        *scrollingVBox // left: info + filters
	colonistInvScroll     *scrollingVBox // right top: carried items
	colonistStorageList   *minui.ListBox // right bottom: collective storage
	colonistStorageBPs    []string
	colonistStorageEntity *ecs.Entity

	// Goals tab
	goalsVBox     *minui.VBox
	goalsPanel    *minui.Panel
	goalsLabels   []*minui.Label
	lastGoalLines []string

	// Crafting station modal
	craftingModal      *minui.Modal
	craftingRecipeList *minui.ListBox
	craftingQueueList  *minui.ListBox
	craftingRecipeIDs  []string
	craftingStationEnt *ecs.Entity

	// Research station modal
	researchModal      *minui.Modal
	researchTechList   *minui.ListBox
	researchQueueList  *minui.ListBox
	researchTechKeys   []string
	researchStationEnt *ecs.Entity

	// HUD elements
	messagesTextArea *minui.ScrollingTextArea
	resourceBar      *minui.ResourceBar

	// Main menu modal
	mainMenuModal *minui.Modal

	// Save/Load modals
	saveModal   *minui.Modal
	loadModal   *minui.Modal
	loadListBox *minui.ListBox
	saveNames   []string

	// Entity detail panel
	detailsPanel   *minui.Panel
	detailsVBox    *minui.VBox
	hoveredEntity  *ecs.Entity
	selectedEntity *ecs.Entity

	// Cursor
	CursorImage *ebiten.Image

	// Tooltips
	tooltipManager       *minui.TooltipManager
	listTooltip          *minui.Tooltip
	craftTooltipDescs    []string
	researchTooltipDescs []string
	storageTooltipDescs  []string

	// Active selection indicator
	selectionTooltip *minui.Tooltip
}

func NewHUDScreen() *HUDScreen {
	theme := minui.NewDarkTheme()
	tm := minui.NewTooltipManager()
	tm.SetTheme(theme)
	tm.SetGlobalPosition(minui.TooltipRight)
	tm.SetGlobalOffset(8)
	h := &HUDScreen{
		uiGUI:          minui.NewGUIWithTheme(theme),
		op:             &ebiten.DrawImageOptions{},
		knownTechs:     map[string]bool{},
		tooltipManager: tm,
	}

	listTT := minui.NewTooltip("listTooltip")
	listTT.Position = minui.TooltipMouse
	listTT.Delay = 15
	listTT.Offset = 40
	listTT.SetTheme(theme)
	h.listTooltip = listTT
	h.setupHUDElements()
	h.setupSidebar()
	h.setupModals()
	h.registerListeners()
	return h
}

func (h *HUDScreen) OnEnter()       {}
func (h *HUDScreen) OnExit()        {}
func (h *HUDScreen) IsOpaque() bool { return true }

func (h *HUDScreen) SetInputBlocked(blocked bool) { h.inputBlocked = blocked }

func (h *HUDScreen) Update() {
	if !h.inputBlocked {
		h.uiGUI.Update()
	}
	h.uiGUI.Layout()
	h.tooltipManager.Update()
	h.listTooltip.Update()
	h.selectionTooltip.Update()
}

func (h *HUDScreen) Draw(screen *ebiten.Image) {
	h.uiGUI.Draw(screen)
	h.tooltipManager.Draw(screen)
	h.listTooltip.Draw(screen)
	h.selectionTooltip.Draw(screen)
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
	h.resourceBar.AddResource("food", nil, 0)
	h.uiGUI.AddElement(h.resourceBar)

	theme := h.uiGUI.GetTheme()
	h.selectionTooltip = minui.NewTooltip("selectionTooltip")
	h.selectionTooltip.SetTheme(theme)
	h.selectionTooltip.SetPosition(210, 8)
	h.selectionTooltip.Hide()

	h.rogueExitButton = minui.NewButton("rogueExit", "Exit Control Mode (X)")
	h.rogueExitButton.SetBounds(minui.Rect{X: sw/2 - 90, Y: sh - 70, Width: 180, Height: 32})
	h.rogueExitButton.OnClick = func() {
		event.GetQueuedInstance().QueueEvent(ExitRogueModeEvent{})
	}
	h.rogueExitButton.SetVisible(false)
	h.uiGUI.AddElement(h.rogueExitButton)
}

// SetSidebarVisible shows or hides the left build/population/goals sidebar.
func (h *HUDScreen) SetSidebarVisible(v bool) {
	if h.sidebarTabPanel != nil {
		h.sidebarTabPanel.SetVisible(v)
	}
}

// ShowRogueExit reveals the on-screen "Exit Control Mode" button.
func (h *HUDScreen) ShowRogueExit() {
	if h.rogueExitButton != nil {
		h.rogueExitButton.SetVisible(true)
	}
}

// HideRogueExit hides the on-screen "Exit Control Mode" button.
func (h *HUDScreen) HideRogueExit() {
	if h.rogueExitButton != nil {
		h.rogueExitButton.SetVisible(false)
	}
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
	id          string
	label       string
	description string
	mode        CursorModeType
}

func (h *HUDScreen) setupBuildTab(panel *minui.Panel) {
	h.buildMenuItems = make(map[string]*minui.MenuItem)

	const itemH = 24
	const panelW = 192

	// Each category's `items` (cursor-mode pseudo-buildables) stays in code
	// because it binds to CursorMode constants. Buildables themselves are
	// pulled from build.json by category — adding a new wall/floor/door is a
	// pure JSON edit (set "category" + optional "order").
	type buildCategory struct {
		id    string
		label string
		items []hudBuildOrderItem
	}

	categories := []buildCategory{
		{
			id:    "orders",
			label: "Orders",
			items: []hudBuildOrderItem{
				{id: "default", label: "Default", description: "Default cursor mode. Select and inspect entities.", mode: CursorModeDefault},
				{id: "dig", label: "Dig", description: "Order colonists to dig through terrain.", mode: CursorModeDig},
				{id: "mine", label: "Mine", description: "Order colonists to mine ore deposits.", mode: CursorModeMine},
				{id: "cancel", label: "Cancel Task", description: "Cancel a pending construction or mining order.", mode: CursorModeCancel},
				{id: "attack", label: "Attack", description: "Order colonists to attack the target.", mode: CursorModeAttack},
				{id: "sleep", label: "Sleep", description: "Order a colonist to rest in a bed and restore health.", mode: CursorModeSleep},
			},
		},
		{id: "walls", label: "Walls"},
		{id: "floors", label: "Floors"},
		{id: "stairs", label: "Stairs"},
		{id: "structures", label: "Structures"},
		{id: "doors", label: "Doors"},
	}

	h.buildCategoryPanel = minui.NewPanel("buildCategories")
	h.buildCategoryPanel.SetBounds(minui.Rect{X: 0, Y: 0, Width: panelW, Height: 600})

	h.buildSubPanel = minui.NewPanel("buildSubMenu")
	h.buildSubPanel.SetBounds(minui.Rect{X: 0, Y: 0, Width: panelW, Height: 600})
	h.buildSubPanel.SetVisible(false)

	showSubMenu := func(title string, items []hudBuildOrderItem, categoryID string) {
		// Buildables are looked up by category at render time, so changes to
		// build.json (after a restart) flow through without code edits.
		types := construction.BuildableTypesInCategory(categoryID)
		h.currentSubTitle = title
		h.currentSubItems = items
		h.currentSubCategory = categoryID

		snapshot := make([]minui.Element, len(h.buildSubPanel.GetChildren()))
		copy(snapshot, h.buildSubPanel.GetChildren())
		for _, child := range snapshot {
			h.buildSubPanel.RemoveChild(child)
		}
		h.tooltipManager.Clear()

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
			oiLabel := oi.label
			oiDesc := oi.description
			mi.OnClick = func() {
				h.selectBuildItem(oiID)
				event.GetQueuedInstance().QueueEvent(CursorModeChangedEvent{Mode: oiMode})
				if oiMode == CursorModeDefault {
					h.setSelectionLabel("", "", nil)
				} else {
					h.setSelectionLabel(oiLabel, oiDesc, nil)
				}
			}
			if oi.description != "" {
				h.tooltipManager.RegisterWithTitle(mi, oi.label, oi.description)
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
			locked := buildable.RequiredTech != "" && !h.knownTechs[buildable.RequiredTech]
			label := buildable.Name
			if locked {
				label = fmt.Sprintf("%s  (requires %s)", buildable.Name, buildable.RequiredTech)
			}
			mi := minui.NewMenuItem("build_"+buildType, label)
			mi.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
			if locked {
				mi.SetEnabled(false)
			} else {
				btName := buildable.Name
				btDesc := buildable.Description
				btIcon := buildableTooltipIcon(buildType, buildable)
				mi.OnClick = func() {
					h.selectBuildItem("build_" + btID)
					event.GetQueuedInstance().QueueEvent(BuildOptionChangedEvent{Option: btID})
					event.GetQueuedInstance().QueueEvent(CursorModeChangedEvent{Mode: CursorModeBuild})
					h.setSelectionLabel(btName, btDesc, btIcon)
				}
			}
			if buildable.Description != "" || buildable.Name != "" {
				h.tooltipManager.Register(mi, buildable.Name, buildable.Description, buildableTooltipIcon(buildType, buildable))
			}
			h.buildMenuItems["build_"+buildType] = mi
			h.buildSubPanel.AddChild(mi)
			y += itemH
		}

		h.buildCategoryPanel.SetVisible(false)
		h.buildSubPanel.SetVisible(true)
	}

	h.showSubMenuFn = showSubMenu

	y := 4
	for _, cat := range categories {
		catCopy := cat
		mi := minui.NewMenuItem("cat_"+cat.id, cat.label+" >")
		mi.SetBounds(minui.Rect{X: 4, Y: y, Width: panelW - 8, Height: itemH})
		mi.OnClick = func() {
			showSubMenu(catCopy.label, catCopy.items, catCopy.id)
		}
		h.buildCategoryPanel.AddChild(mi)
		y += itemH
	}

	panel.AddChild(h.buildCategoryPanel)
	panel.AddChild(h.buildSubPanel)
}

// CraftQueueEntry carries the data needed to render one active craft job.
type CraftQueueEntry struct {
	Name     string
	Sprite   *ebiten.Image // unused; retained for callsite compatibility
	Progress float64       // 0..1
}

// RefreshCraftQueue updates the queue ListBox in the crafting modal.
func (h *HUDScreen) RefreshCraftQueue(entries []CraftQueueEntry) {
	if h.craftingQueueList == nil {
		return
	}
	items := make([]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, fmt.Sprintf("%s  %d%%", e.Name, int(e.Progress*100)))
	}
	h.craftingQueueList.Items = items
	if h.craftingQueueList.SelectedIndex >= len(items) {
		h.craftingQueueList.SelectedIndex = -1
	}
}

func (h *HUDScreen) selectBuildItem(selectedID string) {
	for id, item := range h.buildMenuItems {
		item.SetSelected(id == selectedID)
	}
}

// SetSelectionLabel directly sets the selection tooltip text regardless of cursor mode.
// Pass an empty title to hide it.
func (h *HUDScreen) SetSelectionLabel(title, desc string) {
	h.setSelectionLabel(title, desc, nil)
}

// SetDefaultContext shows a context hint in the selection tooltip while in Default cursor mode.
// Pass an empty title to hide it. blueprint is used to build the sprite icon (may be empty).
func (h *HUDScreen) SetDefaultContext(title, desc, blueprint string) {
	var icon *minui.Icon
	if blueprint != "" {
		icon = blueprintTooltipIcon(blueprint)
	}
	h.setSelectionLabel(title, desc, icon)
}

func (h *HUDScreen) setSelectionLabel(title, desc string, icon *minui.Icon) {
	if h.selectionTooltip == nil {
		return
	}
	if title == "" {
		h.selectionTooltip.Hide()
		return
	}
	h.selectionTooltip.SetContent(title, desc, icon)
	h.selectionTooltip.Layout()
	h.selectionTooltip.SetPosition(210, 8)
	h.selectionTooltip.Show()
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

	h.mainMenuModal = minui.NewModal("mainMenu", "Main Menu", 300, 320)
	h.mainMenuModal.SetPosition(sw/2-150, sh/2-160)
	h.mainMenuModal.SetVisible(false)
	h.mainMenuModal.Closeable = true

	menuVBox := minui.NewVBox("mainMenuButtons")
	menuVBox.SetPosition(30, 50)
	menuVBox.Spacing = 10
	for _, entry := range []struct{ id, label string }{
		{"save", "Save Game"},
		{"load", "Load Game"},
		{"newgame", "New Game"},
		{"quit", "Quit"},
	} {
		action := entry.id
		btn := minui.NewButton(entry.id, entry.label)
		btn.OnClick = func() {
			switch action {
			case "save":
				h.mainMenuModal.SetVisible(false)
				h.saveModal.SetVisible(true)
			case "load":
				h.mainMenuModal.SetVisible(false)
				h.openLoadModal()
			default:
				event.GetQueuedInstance().QueueEvent(MainMenuEvent{Action: action})
			}
		}
		menuVBox.AddChild(btn)
	}
	h.mainMenuModal.AddChild(menuVBox)
	h.uiGUI.AddModal(h.mainMenuModal)

	// Save modal
	h.saveModal = minui.NewModal("saveModal", "Save Game", 300, 160)
	h.saveModal.SetPosition(sw/2-150, sh/2-80)
	h.saveModal.SetVisible(false)
	h.saveModal.Closeable = true

	saveVBox := minui.NewVBox("saveButtons")
	saveVBox.SetPosition(30, 50)
	saveVBox.Spacing = 10
	saveConfirm := minui.NewButton("saveConfirm", "Save")
	saveConfirm.OnClick = func() {
		h.saveModal.SetVisible(false)
		event.GetQueuedInstance().QueueEvent(SaveGameEvent{})
	}
	saveVBox.AddChild(saveConfirm)
	saveCancel := minui.NewButton("saveCancel", "Cancel")
	saveCancel.OnClick = func() {
		h.saveModal.SetVisible(false)
	}
	saveVBox.AddChild(saveCancel)
	h.saveModal.AddChild(saveVBox)
	h.uiGUI.AddModal(h.saveModal)

	// Load modal
	h.loadModal = minui.NewModal("loadModal", "Load Game", 340, 300)
	h.loadModal.SetPosition(sw/2-170, sh/2-150)
	h.loadModal.SetVisible(false)
	h.loadModal.Closeable = true

	h.loadListBox = minui.NewListBox("loadList", nil)
	h.loadListBox.SetBounds(minui.Rect{X: 20, Y: 50, Width: 300, Height: 180})
	h.loadModal.AddChild(h.loadListBox)

	loadVBox := minui.NewVBox("loadButtons")
	loadVBox.SetPosition(20, 240)
	loadVBox.Spacing = 10
	loadConfirm := minui.NewButton("loadConfirm", "Load")
	loadConfirm.OnClick = func() {
		idx := h.loadListBox.SelectedIndex
		if idx >= 0 && idx < len(h.saveNames) {
			name := h.saveNames[idx]
			h.loadModal.SetVisible(false)
			event.GetQueuedInstance().QueueEvent(LoadGameEvent{Name: name})
		}
	}
	loadVBox.AddChild(loadConfirm)
	loadCancel := minui.NewButton("loadCancel", "Cancel")
	loadCancel.OnClick = func() {
		h.loadModal.SetVisible(false)
	}
	loadVBox.AddChild(loadCancel)
	h.loadModal.AddChild(loadVBox)
	h.uiGUI.AddModal(h.loadModal)

	// Crafting station modal — two-column layout: recipes (left) | queue (right)
	const craftModalW, craftModalH = 640, 500
	const colW, colH = 300, 420
	h.craftingModal = minui.NewModal("craftingModal", "", craftModalW, craftModalH)
	h.craftingModal.SetPosition(sw/2-craftModalW/2, sh/2-craftModalH/2)
	h.craftingModal.SetVisible(false)
	h.craftingModal.Closeable = true

	recipeHdr := minui.NewLabel("craftRecipeHdr", "Recipes")
	recipeHdr.SetPosition(14, 40)
	h.craftingModal.AddChild(recipeHdr)

	queueHdr := minui.NewLabel("craftQueueHdr", "Active Queue")
	queueHdr.SetPosition(14+colW+16, 40)
	h.craftingModal.AddChild(queueHdr)

	h.craftingRecipeList = minui.NewListBox("craftRecipeList", nil)
	h.craftingRecipeList.SetBounds(minui.Rect{X: 14, Y: 60, Width: colW, Height: colH})
	h.craftingRecipeList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(h.craftingRecipeIDs) {
			return
		}
		recipeID := h.craftingRecipeIDs[idx]
		if recipeID == "" {
			h.craftingRecipeList.SelectedIndex = -1
			return // locked recipe — not selectable
		}
		event.GetQueuedInstance().QueueEvent(CraftRequestedEvent{
			RecipeID: recipeID,
			Station:  h.craftingStationEnt,
		})
		h.craftingRecipeList.SelectedIndex = -1
	}
	h.craftingModal.AddChild(h.craftingRecipeList)

	h.craftingQueueList = minui.NewListBox("craftQueueList", nil)
	h.craftingQueueList.SetBounds(minui.Rect{X: 14 + colW + 16, Y: 60, Width: colW, Height: colH})
	h.craftingModal.AddChild(h.craftingQueueList)

	h.uiGUI.AddModal(h.craftingModal)

	// Research station modal — tech list (left) | active queue (right)
	const rModalW, rModalH = 640, 500
	const rColW, rColH = 300, 420
	h.researchModal = minui.NewModal("researchModal", "Research", rModalW, rModalH)
	h.researchModal.SetPosition(sw/2-rModalW/2, sh/2-rModalH/2)
	h.researchModal.SetVisible(false)
	h.researchModal.Closeable = true

	techHdr := minui.NewLabel("researchTechHdr", "Tech")
	techHdr.SetPosition(14, 40)
	h.researchModal.AddChild(techHdr)

	rQueueHdr := minui.NewLabel("researchQueueHdr", "Completed / Active")
	rQueueHdr.SetPosition(14+rColW+16, 40)
	h.researchModal.AddChild(rQueueHdr)

	h.researchTechList = minui.NewListBox("researchTechList", nil)
	h.researchTechList.SetBounds(minui.Rect{X: 14, Y: 60, Width: rColW, Height: rColH})
	h.researchTechList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(h.researchTechKeys) {
			return
		}
		techKey := h.researchTechKeys[idx]
		h.researchTechList.SelectedIndex = -1
		if techKey == "" {
			return // completed/in-progress rows are not actionable
		}
		event.GetQueuedInstance().QueueEvent(ResearchRequestedEvent{
			TechKey: techKey,
			Station: h.researchStationEnt,
		})
	}
	h.researchModal.AddChild(h.researchTechList)

	h.researchQueueList = minui.NewListBox("researchQueueList", nil)
	h.researchQueueList.SetBounds(minui.Rect{X: 14 + rColW + 16, Y: 60, Width: rColW, Height: rColH})
	h.researchModal.AddChild(h.researchQueueList)

	h.uiGUI.AddModal(h.researchModal)

	// Colonist modal — equipment (left) | storage (right, scrollable)
	const colModalW, colModalH = 640, 560
	const cColW, cColH = 300, 480
	h.colonistModal = minui.NewModal("colonistModal", "Colonist", colModalW, colModalH)
	h.colonistModal.SetPosition(sw/2-colModalW/2, sh/2-colModalH/2)
	h.colonistModal.SetVisible(false)
	h.colonistModal.Closeable = true

	h.colonistScroll = newScrollingVBox("colonistEquip", 4)
	h.colonistScroll.SetBounds(minui.Rect{X: 14, Y: 40, Width: 300, Height: 480})
	// spacing set in newScrollingVBox
	h.colonistModal.AddChild(h.colonistScroll)

	const rX = 14 + cColW + 16
	// right column: Y=40..520 (matches left column height)
	// top half: carrying inventory (Y=58..258)
	// bottom half: collective storage (Y=286..520)

	invHdr := minui.NewLabel("colonistInvHdr", "Carrying")
	invHdr.SetPosition(rX, 40)
	h.colonistModal.AddChild(invHdr)

	h.colonistInvScroll = newScrollingVBox("colonistInv", 4)
	h.colonistInvScroll.SetBounds(minui.Rect{X: rX, Y: 58, Width: cColW, Height: 200})
	h.colonistModal.AddChild(h.colonistInvScroll)

	storageHdr := minui.NewLabel("colonistStorageHdr", "Available in Storage")
	storageHdr.SetPosition(rX, 268)
	h.colonistModal.AddChild(storageHdr)

	h.colonistStorageList = minui.NewListBox("colonistStorageList", nil)
	h.colonistStorageList.SetBounds(minui.Rect{X: rX, Y: 286, Width: cColW, Height: 234})
	h.colonistStorageList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(h.colonistStorageBPs) || h.colonistStorageEntity == nil {
			return
		}
		event.GetQueuedInstance().QueueEvent(EquipItemRequestedEvent{
			ColonistEntity: h.colonistStorageEntity,
			ItemBlueprint:  h.colonistStorageBPs[idx],
		})
		h.colonistStorageList.SelectedIndex = -1
		h.colonistModal.SetVisible(false)
	}
	h.colonistModal.AddChild(h.colonistStorageList)

	h.uiGUI.AddModal(h.colonistModal)
}

func (h *HUDScreen) registerListeners() {
	event.GetQueuedInstance().RegisterListener(h, message.MessageEventType)
	event.GetQueuedInstance().RegisterListener(h, EntitySelectedEventType)
	event.GetQueuedInstance().RegisterListener(h, minui.EventTypeModalClose)
	event.GetQueuedInstance().RegisterListener(h, CursorModeChangedEventType)
}

func (h *HUDScreen) HandleEvent(evt event.EventData) error {
	switch e := evt.(type) {
	case message.MessageEvent:
		h.messagesTextArea.AddText(fmt.Sprintf("%s: %s", e.Sender, e.Message))
	case EntitySelectedEvent:
		_ = e
	case minui.ModalCloseEvent:
		_ = e
	case CursorModeChangedEvent:
		if e.Mode == CursorModeDefault {
			h.selectBuildItem("default")
			h.setSelectionLabel("", "", nil)
		}
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

	// Header label
	if h.populationHeader == nil {
		h.populationHeader = minui.NewLabel("popHeader", "")
		h.populationHeader.GetStyle().FontSize = &fontSize
		h.populationVBox.AddChild(h.populationHeader)
	}
	h.populationHeader.Text = fmt.Sprintf("Colonists: %d", len(entries))

	// Grow items slice (name row + a Take Control row per colonist)
	for len(h.populationItems) < len(entries) {
		idx := len(h.populationItems)
		mi := minui.NewMenuItem(fmt.Sprintf("pop_%d", idx), "")
		ctrl := minui.NewMenuItem(fmt.Sprintf("pop_ctrl_%d", idx), "  → Take Control")
		ctrlSize := 11
		ctrl.GetStyle().FontSize = &ctrlSize
		h.populationItems = append(h.populationItems, mi)
		h.populationCtrl = append(h.populationCtrl, ctrl)
		h.populationVBox.AddChild(mi)
		h.populationVBox.AddChild(ctrl)
	}
	// Shrink items slice
	for len(h.populationItems) > len(entries) {
		last := h.populationItems[len(h.populationItems)-1]
		lastCtrl := h.populationCtrl[len(h.populationCtrl)-1]
		h.populationVBox.RemoveChild(last)
		h.populationVBox.RemoveChild(lastCtrl)
		h.populationItems = h.populationItems[:len(h.populationItems)-1]
		h.populationCtrl = h.populationCtrl[:len(h.populationCtrl)-1]
	}

	for i, entry := range entries {
		captured := entry
		h.populationItems[i].Text = fmt.Sprintf("%s  [%s/%s]", entry.Name, entry.State, entry.Task)
		h.populationItems[i].OnClick = func() {
			event.GetQueuedInstance().QueueEvent(ColonistSelectedEvent{Entity: captured.Entity})
		}
		h.populationCtrl[i].OnClick = func() {
			event.GetQueuedInstance().QueueEvent(EnterRogueModeEvent{Entity: captured.Entity})
		}
	}
}

// StorageItemEntry represents an equippable item in settlement storage for the colonist modal.
type StorageItemEntry struct {
	Blueprint string
	Name      string
	Slot      string
}

func (h *HUDScreen) ShowColonistModal(colonist *ecs.Entity, storageItems []StorageItemEntry) {
	if h.colonistModal == nil || colonist == nil {
		return
	}

	for _, child := range append([]minui.Element{}, h.colonistScroll.GetContent()...) {
		h.colonistScroll.RemoveContent(child)
	}
	for _, child := range append([]minui.Element{}, h.colonistInvScroll.GetContent()...) {
		h.colonistInvScroll.RemoveContent(child)
	}
	h.tooltipManager.Clear()

	inv := colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	dc := colonist.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)

	// Populate the carrying panel
	capturedColonist := colonist
	if len(inv.Bag) == 0 {
		empty := minui.NewLabel("inv_empty", "(nothing)")
		h.colonistInvScroll.AddContent(empty)
	} else {
		for i, item := range inv.Bag {
			name := item.Blueprint
			if item.HasComponent(rlcomponents.Description) {
				name = item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			if item.HasComponent(components.ResourceItem) {
				rc := item.GetComponent(components.ResourceItem).(*components.ResourceItemComponent)
				name = fmt.Sprintf("%s x%d", name, rc.Quantity)
			}
			mi := minui.NewMenuItem(fmt.Sprintf("inv_%d", i), fmt.Sprintf("%s  [Drop Off]", name))
			mi.SetBounds(minui.Rect{X: 0, Y: 0, Width: 290, Height: 22})
			capturedItem := item
			mi.OnClick = func() {
				event.GetQueuedInstance().QueueEvent(DropOffRequestedEvent{Colonist: capturedColonist, Item: capturedItem})
				h.colonistModal.SetVisible(false)
			}
			if ttTitle, ttDesc := itemTooltipText(item); ttTitle != "" || ttDesc != "" {
				h.tooltipManager.Register(mi, ttTitle, ttDesc, itemTooltipIcon(item))
			}
			h.colonistInvScroll.AddContent(mi)
		}
	}

	fontSize := 13
	smallSize := 11

	nameLabel := minui.NewLabel("colonistName", dc.Name)
	nameLabel.GetStyle().FontSize = &fontSize
	h.colonistScroll.AddContent(nameLabel)

	rogueBtn := minui.NewMenuItem("colonistTakeControl", "[ Take Control (Rogue) ]")
	rogueBtn.SetBounds(minui.Rect{X: 0, Y: 0, Width: 290, Height: 22})
	rogueBtn.OnClick = func() {
		event.GetQueuedInstance().QueueEvent(EnterRogueModeEvent{Entity: capturedColonist})
		h.colonistModal.SetVisible(false)
	}
	h.colonistScroll.AddContent(rogueBtn)

	atk := inv.GetAttackModifier()
	def := inv.GetDefenseModifier()
	statsLabel := minui.NewLabel("colonistStats", fmt.Sprintf("ATK: %+d   DEF: %+d", atk, def))
	statsLabel.GetStyle().FontSize = &smallSize
	h.colonistScroll.AddContent(statsLabel)

	if colonist.HasComponent(components.StatProgression) {
		prog := colonist.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
		levelStr := fmt.Sprintf("Str Lv.%d  Dex Lv.%d  Int Lv.%d  Con Lv.%d",
			prog.Str.Level, prog.Dex.Level, prog.Int.Level, prog.Con.Level)
		levelsLabel := minui.NewLabel("colonistStatLevels", levelStr)
		levelsLabel.GetStyle().FontSize = &smallSize
		h.colonistScroll.AddContent(levelsLabel)
	}

	eqHdr := minui.NewLabel("eqHdr", "── Equipment ──")
	eqHdr.GetStyle().FontSize = &smallSize
	h.colonistScroll.AddContent(eqHdr)

	slotList := []struct {
		label string
		slot  rlcomponents.ItemSlot
		item  *ecs.Entity
	}{
		{"Right Hand", rlcomponents.HandSlot, inv.RightHand},
		{"Left Hand", rlcomponents.OffHandSlot, inv.LeftHand},
		{"Head", rlcomponents.HeadSlot, inv.Head},
		{"Torso", rlcomponents.TorsoSlot, inv.Torso},
		{"Legs", rlcomponents.LegsSlot, inv.Legs},
		{"Feet", rlcomponents.FeetSlot, inv.Feet},
	}

	const rowW, rowH = 300, 22
	for _, sl := range slotList {
		captured := sl
		itemName := "(empty)"
		if captured.item != nil && captured.item.HasComponent(rlcomponents.Description) {
			itemName = captured.item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
		}
		rowText := fmt.Sprintf("%s: %s", captured.label, itemName)
		if captured.item != nil {
			mi := minui.NewMenuItem(fmt.Sprintf("unequip_%s", captured.label), rowText+"  [Unequip]")
			mi.SetBounds(minui.Rect{X: 0, Y: 0, Width: rowW, Height: rowH})
			mi.OnClick = func() {
				event.GetQueuedInstance().QueueEvent(UnequipItemRequestedEvent{
					ColonistEntity: colonist,
					Slot:           string(captured.slot),
				})
				h.colonistModal.SetVisible(false)
			}
			if ttTitle, ttDesc := itemTooltipText(captured.item); ttTitle != "" || ttDesc != "" {
				h.tooltipManager.Register(mi, ttTitle, ttDesc, itemTooltipIcon(captured.item))
			}
			h.colonistScroll.AddContent(mi)
		} else {
			lbl := minui.NewLabel(fmt.Sprintf("slot_%s", captured.label), rowText)
			lbl.GetStyle().FontSize = &smallSize
			lbl.SetSize(rowW, rowH)
			h.colonistScroll.AddContent(lbl)
		}
	}

	// Task filter toggles
	filterHdr := minui.NewLabel("filterHdr", "── Task Filters ──")
	filterHdr.GetStyle().FontSize = &smallSize
	h.colonistScroll.AddContent(filterHdr)

	if colonist.HasComponent(components.Worker) {
		wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
		capturedColonistSD := colonist
		sdToggle := minui.NewToggle("filter_self_defend", "Self-Defend")
		tw, th := 280, 26
		sdToggle.GetStyle().Width = &tw
		sdToggle.GetStyle().Height = &th
		sdToggle.On = wc.SelfDefend
		sdToggle.OnChange = func(on bool) {
			event.GetQueuedInstance().QueueEvent(SetSelfDefendEvent{
				Colonist: capturedColonistSD,
				Enabled:  on,
			})
		}
		h.colonistScroll.AddContent(sdToggle)

		for _, fa := range task_requests.FilterableActions {
			capturedFA := fa
			capturedColonist := colonist
			toggle := minui.NewToggle(fmt.Sprintf("filter_%s", fa.Action), fa.Label)
			tw, th := 280, 26
			toggle.GetStyle().Width = &tw
			toggle.GetStyle().Height = &th
			toggle.On = colonistTaskEnabled(wc, fa)
			toggle.OnChange = func(on bool) {
				event.GetQueuedInstance().QueueEvent(SetTaskFilterEvent{
					Colonist: capturedColonist,
					Action:   string(capturedFA.Action),
					Enabled:  on,
				})
			}
			h.colonistScroll.AddContent(toggle)
		}
	}

	h.colonistStorageEntity = colonist
	items := make([]string, 0, len(storageItems))
	h.colonistStorageBPs = h.colonistStorageBPs[:0]
	h.storageTooltipDescs = h.storageTooltipDescs[:0]
	for _, si := range storageItems {
		items = append(items, si.Name)
		h.colonistStorageBPs = append(h.colonistStorageBPs, si.Blueprint)
		desc := si.Slot
		if desc != "" {
			desc = "Slot: " + desc
		}
		h.storageTooltipDescs = append(h.storageTooltipDescs, desc)
	}
	h.colonistStorageList.SetItems(items)
	h.colonistStorageList.OnHover = func(idx int) {
		if idx < 0 || idx >= len(h.storageTooltipDescs) {
			h.listTooltip.Hide()
			return
		}
		desc := h.storageTooltipDescs[idx]
		var icon *minui.Icon
		if idx < len(h.colonistStorageBPs) {
			icon = blueprintTooltipIcon(h.colonistStorageBPs[idx])
		}
		if desc == "" && icon == nil {
			h.listTooltip.Hide()
			return
		}
		h.listTooltip.SetContent(items[idx], desc, icon)
		h.listTooltip.Show()
	}

	h.colonistModal.SetVisible(true)
}

// colonistTaskEnabled reports whether a task action is currently enabled for the given worker.
// If no filter is set (AllowedTasks is nil), all tasks are considered enabled.
// An empty non-nil slice means all tasks are blocked.
func colonistTaskEnabled(wc *components.WorkerComponent, action task_requests.FilterableAction) bool {
	if wc.AllowedTasks == nil {
		return true
	}
	for _, a := range wc.AllowedTasks {
		if a == action.Action {
			return true
		}
	}
	return false
}

func (h *HUDScreen) OpenCraftingModal(title string, recipes, lockedRecipes []crafting.Recipe, station *ecs.Entity) {
	h.craftingStationEnt = station
	h.craftingModal.Title = title

	total := len(recipes) + len(lockedRecipes)
	items := make([]string, 0, total)
	itemColors := make([]color.Color, 0, total)
	h.craftingRecipeIDs = h.craftingRecipeIDs[:0]
	h.craftTooltipDescs = h.craftTooltipDescs[:0]

	appendRecipe := func(r crafting.Recipe, locked bool) {
		costStr := ""
		for mat, qty := range r.Cost {
			if costStr != "" {
				costStr += ", "
			}
			costStr += fmt.Sprintf("%s×%d", mat, qty)
		}
		var label string
		if locked {
			prereqName := r.RequiresTech
			if prereq, ok := research.GetTech(r.RequiresTech); ok {
				prereqName = prereq.Name
			}
			label = fmt.Sprintf("%s  [requires: %s]", r.Name, prereqName)
			itemColors = append(itemColors, color.RGBA{120, 120, 120, 255})
			h.craftingRecipeIDs = append(h.craftingRecipeIDs, "") // empty = not selectable
		} else {
			if costStr != "" {
				label = fmt.Sprintf("%s  [%s]", r.Name, costStr)
			} else {
				label = r.Name
			}
			itemColors = append(itemColors, nil)
			h.craftingRecipeIDs = append(h.craftingRecipeIDs, r.Output)
		}
		items = append(items, label)
		h.craftTooltipDescs = append(h.craftTooltipDescs, r.Description)
	}

	for _, r := range recipes {
		appendRecipe(r, false)
	}
	for _, r := range lockedRecipes {
		appendRecipe(r, true)
	}

	h.craftingRecipeList.SetItems(items)
	h.craftingRecipeList.ItemColors = itemColors
	h.craftingRecipeList.OnHover = func(idx int) {
		if idx < 0 || idx >= len(h.craftTooltipDescs) {
			h.listTooltip.Hide()
			return
		}
		desc := h.craftTooltipDescs[idx]
		var icon *minui.Icon
		if idx < len(h.craftingRecipeIDs) && h.craftingRecipeIDs[idx] != "" {
			icon = blueprintTooltipIcon(h.craftingRecipeIDs[idx])
		}
		if desc == "" && icon == nil {
			h.listTooltip.Hide()
			return
		}
		h.listTooltip.SetContent(items[idx], desc, icon)
		h.listTooltip.Show()
	}

	h.craftingModal.SetVisible(true)
}

// ResearchQueueEntry is one row in the active-research column.
type ResearchQueueEntry struct {
	Name     string
	Progress float64 // 0..1
}

// OpenResearchModal opens the research modal for a given lab entity.
func (h *HUDScreen) OpenResearchModal(station *ecs.Entity, available, locked, completed, inProgress []research.Tech, queue []ResearchQueueEntry) {
	h.researchStationEnt = station
	h.populateResearchTechList(available, locked)
	h.populateResearchQueueList(completed, queue)
	h.researchModal.SetVisible(true)
}

// RefreshResearchModal updates the modal in place if it's currently visible.
func (h *HUDScreen) RefreshResearchModal(available, locked, completed, inProgress []research.Tech, queue []ResearchQueueEntry) {
	if h.researchModal == nil || !h.researchModal.IsVisible() {
		return
	}
	h.populateResearchTechList(available, locked)
	h.populateResearchQueueList(completed, queue)
}

func (h *HUDScreen) populateResearchTechList(available, locked []research.Tech) {
	items := make([]string, 0, len(available)+len(locked))
	keys := make([]string, 0, len(available)+len(locked))
	colors := make([]color.Color, 0, len(available)+len(locked))
	h.researchTooltipDescs = h.researchTooltipDescs[:0]
	for _, t := range available {
		items = append(items, fmt.Sprintf("%s  [%d ticks]", t.Name, t.Duration))
		keys = append(keys, t.Key)
		colors = append(colors, nil)
		h.researchTooltipDescs = append(h.researchTooltipDescs, t.Description)
	}
	for _, t := range locked {
		prereqName := t.RequiresTech
		if prereq, ok := research.GetTech(t.RequiresTech); ok {
			prereqName = prereq.Name
		}
		items = append(items, fmt.Sprintf("%s  [requires: %s]", t.Name, prereqName))
		keys = append(keys, "") // empty key = not selectable
		colors = append(colors, color.RGBA{120, 120, 120, 255})
		h.researchTooltipDescs = append(h.researchTooltipDescs, t.Description)
	}
	if len(items) == 0 {
		items = append(items, "No research available.")
		keys = append(keys, "")
		colors = append(colors, nil)
		h.researchTooltipDescs = append(h.researchTooltipDescs, "")
	}
	h.researchTechList.SetItems(items)
	h.researchTechList.ItemColors = colors
	h.researchTechKeys = keys
	h.researchTechList.OnHover = func(idx int) {
		if idx < 0 || idx >= len(h.researchTooltipDescs) {
			h.listTooltip.Hide()
			return
		}
		desc := h.researchTooltipDescs[idx]
		if desc == "" {
			h.listTooltip.Hide()
			return
		}
		h.listTooltip.SetContent(items[idx], desc, nil)
		h.listTooltip.Show()
	}
}

// populateResearchQueueList shows active research at the top, completed below.
func (h *HUDScreen) populateResearchQueueList(completed []research.Tech, queue []ResearchQueueEntry) {
	items := make([]string, 0, len(queue)+len(completed))
	for _, e := range queue {
		items = append(items, fmt.Sprintf("%s  %d%%", e.Name, int(e.Progress*100)))
	}
	for _, t := range completed {
		items = append(items, fmt.Sprintf("%s  (done)", t.Name))
	}
	h.researchQueueList.Items = items
	if h.researchQueueList.SelectedIndex >= len(items) {
		h.researchQueueList.SelectedIndex = -1
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

// HoveredTileInfo carries the data shown in the tile-hover detail panel.
//
// Name is the Middle slot's tile name (e.g. "air", "rock"). FloorName is the
// Floor slot's tile name when present (e.g. "rock_floor", "regolith"). The
// hover panel prefers FloorName whenever the Middle is air/empty, so a player
// hovering a cavern reads "rock_floor" instead of the meaningless "air".
type HoveredTileInfo struct {
	Name                     string
	FloorName                string
	X, Y, Z                  int
	LightLevel               int
	Radiation                int
	Solid, Water, Air, Space bool
}

// SetHoveredTile shows tile info in the detail panel when no entity is under the cursor.
func (h *HUDScreen) SetHoveredTile(info HoveredTileInfo) {
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

	// Prefer the Floor name when standing on something through air/empty
	// (caverns, surface). Falls back to Middle for solids, atmosphere, space.
	displayName := info.Name
	if (info.Air || info.Name == "" || info.Name == "air") && info.FloorName != "" {
		displayName = info.FloorName
	}
	add("Tile: " + displayName)
	add(fmt.Sprintf("Pos: %d, %d, %d", info.X, info.Y, info.Z))
	add(fmt.Sprintf("Light: %d", info.LightLevel))
	if info.Radiation > 0 {
		add(fmt.Sprintf("Radiation: %d", info.Radiation))
	}
	flags := ""
	if info.Solid {
		flags += " solid"
	}
	if info.Water {
		flags += " water"
	}
	if info.Air {
		flags += " air"
	}
	if info.Space {
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

func (h *HUDScreen) GetInputFocused() bool           { return h.uiGUI.GetKeyboardFocused() }
func (h *HUDScreen) GetMouseFocused() bool           { return h.uiGUI.GetMouseFocused() }
func (h *HUDScreen) WithinModalBounds(x, y int) bool { return h.uiGUI.WithinModalBounds(x, y) }

func (h *HUDScreen) openLoadModal() {
	h.loadModal.SetVisible(true)
}

func (h *HUDScreen) SetKnownTechs(techs []string) {
	newSet := make(map[string]bool, len(techs))
	for _, t := range techs {
		newSet[t] = true
	}
	changed := len(newSet) != len(h.knownTechs)
	if !changed {
		for k := range newSet {
			if !h.knownTechs[k] {
				changed = true
				break
			}
		}
	}
	h.knownTechs = newSet
	if changed && h.buildSubPanel != nil && h.buildSubPanel.IsVisible() && h.showSubMenuFn != nil {
		h.showSubMenuFn(h.currentSubTitle, h.currentSubItems, h.currentSubCategory)
	}
}

func (h *HUDScreen) SetSaveNames(names []string) {
	h.saveNames = names
	items := make([]string, len(names))
	copy(items, names)
	h.loadListBox.Items = items
	h.loadListBox.SelectedIndex = -1
}

func (h *HUDScreen) OpenModal(name string) {
	switch name {
	case "mainMenu":
		h.mainMenuModal.SetVisible(true)
	case "saveModal":
		h.saveModal.SetVisible(true)
	case "loadModal":
		h.openLoadModal()
	}
}

// CloseColonistModal hides the colonist detail modal and clears its hover
// tooltips so they don't linger (e.g. when entering Rogue mode).
func (h *HUDScreen) CloseColonistModal() {
	if h.colonistModal != nil {
		h.colonistModal.SetVisible(false)
	}
	h.tooltipManager.Clear()
}

func (h *HUDScreen) CloseModal(name string) {
	switch name {
	case "colonistModal":
		h.CloseColonistModal()
	case "mainMenu":
		h.mainMenuModal.SetVisible(false)
	case "saveModal":
		h.saveModal.SetVisible(false)
	case "loadModal":
		h.loadModal.SetVisible(false)
	}
}

func (h *HUDScreen) ModalOpen(name string) bool {
	switch name {
	case "mainMenu":
		return h.mainMenuModal.IsVisible()
	case "saveModal":
		return h.saveModal.IsVisible()
	case "loadModal":
		return h.loadModal.IsVisible()
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
		if entity.HasComponent(components.StatProgression) {
			prog := entity.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
			add(fmt.Sprintf("Str:%d[%d] Dex:%d[%d] Int:%d[%d] AC:%d",
				sc.Str, prog.Str.Level, sc.Dex, prog.Dex.Level, sc.Int, prog.Int.Level, sc.AC))
		} else {
			add(fmt.Sprintf("Str:%d Dex:%d Int:%d AC:%d", sc.Str, sc.Dex, sc.Int, sc.AC))
		}
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
	if entity.HasComponent(components.Hunger) {
		hg := entity.GetComponent(components.Hunger).(*components.HungerComponent)
		status := "fed"
		if hg.IsStarving() {
			status = "STARVING"
		} else if hg.IsHungry() {
			status = "hungry"
		}
		add(fmt.Sprintf("Hunger: %d/%d (%s)", hg.Energy, hg.MaxEnergy, status))
	}
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

		equippedSlots := []struct {
			label string
			item  *ecs.Entity
		}{
			{"RH", inv.RightHand},
			{"LH", inv.LeftHand},
			{"Head", inv.Head},
			{"Torso", inv.Torso},
			{"Legs", inv.Legs},
			{"Feet", inv.Feet},
		}
		hasEquipment := false
		for _, sl := range equippedSlots {
			if sl.item != nil {
				hasEquipment = true
				break
			}
		}
		if hasEquipment {
			add("── Equipment ──")
			for _, sl := range equippedSlots {
				if sl.item == nil {
					continue
				}
				name := sl.item.Blueprint
				if d := sl.item.GetComponent(rlcomponents.Description); d != nil {
					name = d.(*rlcomponents.DescriptionComponent).Name
				}
				add(fmt.Sprintf("  %s: %s", sl.label, name))
			}
		}

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
				name := item.Blueprint
				if d := item.GetComponent(rlcomponents.Description); d != nil {
					name = d.(*rlcomponents.DescriptionComponent).Name
				}
				if item.HasComponent(components.ResourceItem) {
					rc := item.GetComponent(components.ResourceItem).(*components.ResourceItemComponent)
					add(fmt.Sprintf("  - %s x%d", name, rc.Quantity))
				} else {
					add("  - " + name)
				}
			}
		} else {
			add("Storage: empty")
		}
	}
	if entity.HasComponent(rlcomponents.Door) {
		door := entity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
		state := "closed"
		if door.Open {
			state = "open"
		}
		add("Door: " + state)
	}

	h.resizeDetailPanel()
}

func (h *HUDScreen) SetDetailsTopOffset(offset int) { h.detailsTopOffset = offset }

func (h *HUDScreen) resizeDetailPanel() {
	const panelW = 220
	const padding = 8
	const margin = 8

	h.detailsVBox.Layout()
	panelH := h.detailsVBox.GetHeight() + padding*2

	cfg := config.Global()
	h.detailsPanel.SetSize(panelW, panelH)
	topY := margin + h.detailsTopOffset
	h.detailsPanel.SetPosition(cfg.ScreenWidth-panelW-margin, topY)
	h.detailsVBox.SetPosition(padding, padding)
}

func popEntriesEqual(a, b []PopulationEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].State != b[i].State || a[i].Task != b[i].Task || a[i].Entity != b[i].Entity {
			return false
		}
	}
	return true
}

// itemTooltipText builds a (title, description) pair for a tooltip from an item entity.
// Returns empty strings if the item has no useful info to show.
func itemTooltipText(item *ecs.Entity) (title, desc string) {
	if item == nil {
		return "", ""
	}

	if item.HasComponent(rlcomponents.Description) {
		dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		title = dc.Name
	}

	if item.HasComponent(rlcomponents.Item) {
		ic := item.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent)
		if ic.Description != "" {
			desc += ic.Description + "\n"
		}
	}

	if item.HasComponent(rlcomponents.Food) {
		fc := item.GetComponent(rlcomponents.Food).(*rlcomponents.FoodComponent)
		desc += fmt.Sprintf("Energy: %d\n", fc.Amount)
	}

	if item.HasComponent(rlcomponents.Weapon) {
		wc := item.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
		line := fmt.Sprintf("DMG: %s", wc.AttackDice)
		if wc.AttackBonus != 0 {
			line += fmt.Sprintf("  ATK: %+d", wc.AttackBonus)
		}
		if wc.DamageType != "" {
			line += fmt.Sprintf("  [%s]", wc.DamageType)
		}
		if wc.Ranged {
			line += fmt.Sprintf("  Range: %d", wc.Range)
		}
		desc += line + "\n"
	}

	if item.HasComponent(rlcomponents.Armor) {
		ac := item.GetComponent(rlcomponents.Armor).(*rlcomponents.ArmorComponent)
		line := fmt.Sprintf("DEF: %+d", ac.DefenseBonus)
		if ac.StoppingPower > 0 {
			line += fmt.Sprintf("  SP: %d", ac.StoppingPower)
		}
		desc += line + "\n"
	}

	if item.HasComponent(components.Skills) {
		sc := item.GetComponent(components.Skills).(*components.SkillsComponent)
		if len(sc.Skills) > 0 {
			desc += "Skills: "
			for i, s := range sc.Skills {
				if i > 0 {
					desc += ", "
				}
				desc += s
			}
			desc += "\n"
		}
	}

	desc = strings.TrimRight(desc, "\n")
	return title, desc
}

// itemTooltipIcon returns an icon preview for the given item entity, or nil
// if the entity has no usable Appearance.
func itemTooltipIcon(item *ecs.Entity) *minui.Icon {
	if item == nil || !item.HasComponent(components.Appearance) {
		return nil
	}
	ac := item.GetComponent(components.Appearance).(*components.AppearanceComponent)
	return appearanceIcon(ac)
}

// appearanceIcon builds a tooltip Icon from an AppearanceComponent. Returns
// nil if the component has no resource set.
func appearanceIcon(ac *components.AppearanceComponent) *minui.Icon {
	if ac == nil || ac.Resource == "" {
		return nil
	}
	size := ac.SpriteSize
	if size <= 0 {
		size = 24
	}
	// Scale up small item sprites so they're visible in the tooltip.
	scale := 2.0
	if size >= 24 {
		scale = 1.5
	}
	icon := minui.NewIconWithScale(ac.Resource, ac.SpriteX, ac.SpriteY, size, size, scale)
	if ac.R != 0 || ac.G != 0 || ac.B != 0 {
		icon.Tint = color.RGBA{R: ac.R, G: ac.G, B: ac.B, A: 255}
	}
	return icon
}

// blueprintTooltipIcon builds a tooltip Icon from a blueprint's Appearance,
// without instantiating the entity. Returns nil if the blueprint has no
// Appearance.
func blueprintTooltipIcon(blueprint string) *minui.Icon {
	return appearanceIcon(factory.GetAppearance(blueprint))
}

// buildableTooltipIcon builds a preview Icon for a build menu entry. Entity
// buildables resolve through the blueprint registry; tile buildables resolve
// through the tile definitions table using the first variant's sprite.
func buildableTooltipIcon(key string, b construction.Buildable) *minui.Icon {
	name := b.Type
	if name == "" {
		name = key
	}
	if name == "" {
		return nil
	}
	if b.IsEntity {
		return blueprintTooltipIcon(name)
	}
	idx, ok := world.TileNameToIndex[name]
	if !ok || idx < 0 || idx >= len(world.TileDefinitions) {
		return nil
	}
	def := world.TileDefinitions[idx]
	if def.Resource == "" || len(def.Variants) == 0 {
		return nil
	}
	w := def.SpriteWidth
	h := def.SpriteHeight
	if w <= 0 {
		w = config.Global().TileSizeW
	}
	if h <= 0 {
		h = config.Global().TileSizeH
	}
	if w <= 0 {
		w = 24
	}
	if h <= 0 {
		h = 24
	}
	v := def.Variants[0]
	return minui.NewIconWithScale(def.Resource, v.SpriteX, v.SpriteY, w, h, 1.5)
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
