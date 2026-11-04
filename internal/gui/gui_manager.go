package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/ui/minui"
)

type PopulationEntry struct {
	Name   string
	State  string
	Task   string
	Entity *ecs.Entity
}

// GUIManager is the stable public API used by the rest of the game.
type GUIManager struct {
	screens     *ScreenManager
	hud         *HUDScreen
	CursorImage *ebiten.Image
}

func NewGUIManager() *GUIManager {
	hud := NewHUDScreen()
	sm := &ScreenManager{}
	sm.Push(hud)
	return &GUIManager{screens: sm, hud: hud}
}

// Close detaches the HUD from the global event queue.
func (gm *GUIManager) Close() {
	event.GetQueuedInstance().UnregisterListenerFromAll(gm.hud)
}

func (gm *GUIManager) Update() {
	gm.hud.CursorImage = gm.CursorImage
	gm.screens.Update()
}

func (gm *GUIManager) Draw(screen *ebiten.Image) {
	gm.screens.Draw(screen)
}

func (gm *GUIManager) ActiveSidebarTab() string { return gm.hud.ActiveSidebarTab() }
func (gm *GUIManager) RefreshPopulationTab(entries []PopulationEntry) {
	gm.hud.RefreshPopulationTab(entries)
}
func (gm *GUIManager) RefreshGoalsTab(lines []string)              { gm.hud.RefreshGoalsTab(lines) }
func (gm *GUIManager) RefreshCraftQueue(entries []CraftQueueEntry) { gm.hud.RefreshCraftQueue(entries) }

func (gm *GUIManager) OpenModal(name string)      { gm.hud.OpenModal(name) }
func (gm *GUIManager) CloseModal(name string)     { gm.hud.CloseModal(name) }
func (gm *GUIManager) ModalOpen(name string) bool { return gm.hud.ModalOpen(name) }

// ColonistModalVisible reports whether the colonist panel is up. It is separate
// from ModalOpen because that drives SetInputBlocked, and the panel has to keep
// taking input to be used and closed.
func (gm *GUIManager) ColonistModalVisible() bool { return gm.hud.ColonistModalVisible() }

func (gm *GUIManager) ToggleModal(name string) {
	if gm.hud.ModalOpen(name) {
		gm.hud.CloseModal(name)
	} else {
		gm.hud.OpenModal(name)
	}
}

func (gm *GUIManager) SetInputBlocked(blocked bool)    { gm.hud.SetInputBlocked(blocked) }
func (gm *GUIManager) SetPauseIndicator(text string)   { gm.hud.SetPauseIndicator(text) }
func (gm *GUIManager) PauseIndicator() (bool, string)  { return gm.hud.PauseIndicator() }
func (gm *GUIManager) SetDetailsTopOffset(offset int)  { gm.hud.SetDetailsTopOffset(offset) }
func (gm *GUIManager) GetInputFocused() bool           { return gm.hud.GetInputFocused() }
func (gm *GUIManager) GetMouseFocused() bool           { return gm.hud.GetMouseFocused() }
func (gm *GUIManager) WithinModalBounds(x, y int) bool { return gm.hud.WithinModalBounds(x, y) }

// ElementBounds returns the on-screen rectangle of the HUD element with the
// given ID (a sidebar menu item such as "cat_orders" or "build_storage_locker"),
// and false when it does not exist or it or any ancestor is hidden. The
// tutorial's highlights use it.
func (gm *GUIManager) ElementBounds(id string) (x, y, w, h int, ok bool) {
	return gm.hud.ElementBounds(id)
}

// ModalElement finds an element inside a named HUD modal; see HUDScreen.ModalElement.
func (gm *GUIManager) ModalElement(modal, id string) minui.Element {
	return gm.hud.ModalElement(modal, id)
}

func (gm *GUIManager) SetHoveredEntity(entity *ecs.Entity) { gm.hud.SetHoveredEntity(entity) }
func (gm *GUIManager) SetHoveredTile(info HoveredTileInfo) { gm.hud.SetHoveredTile(info) }
func (gm *GUIManager) ClearHover()                         { gm.hud.ClearHover() }
func (gm *GUIManager) SetDefaultContext(title, desc, blueprint string) {
	gm.hud.SetDefaultContext(title, desc, blueprint)
}
func (gm *GUIManager) SetSelectionLabel(title, desc string) { gm.hud.SetSelectionLabel(title, desc) }
func (gm *GUIManager) SetSidebarVisible(v bool)             { gm.hud.SetSidebarVisible(v) }
func (gm *GUIManager) ShowRogueExit()                       { gm.hud.ShowRogueExit() }
func (gm *GUIManager) HideRogueExit()                       { gm.hud.HideRogueExit() }
func (gm *GUIManager) UpdateResource(id string, value int)  { gm.hud.UpdateResource(id, value) }
func (gm *GUIManager) SetSaveNames(names []string)          { gm.hud.SetSaveNames(names) }
func (gm *GUIManager) SetKnownTechs(techs []string)         { gm.hud.SetKnownTechs(techs) }
func (gm *GUIManager) ShowColonistModal(colonist *ecs.Entity, storageItems []StorageItemEntry) {
	gm.hud.ShowColonistModal(colonist, storageItems)
}

func (gm *GUIManager) OpenCraftingModal(title string, recipes, lockedRecipes []crafting.Recipe, station *ecs.Entity) {
	gm.hud.OpenCraftingModal(title, recipes, lockedRecipes, station)
}

func (gm *GUIManager) OpenResearchModal(station *ecs.Entity, available, locked, completed, inProgress []research.Tech, queue []ResearchQueueEntry) {
	gm.hud.OpenResearchModal(station, available, locked, completed, inProgress, queue)
}

func (gm *GUIManager) RefreshResearchModal(available, locked, completed, inProgress []research.Tech, queue []ResearchQueueEntry) {
	gm.hud.RefreshResearchModal(available, locked, completed, inProgress, queue)
}
