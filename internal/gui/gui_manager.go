package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/mlge/ecs"
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

func (gm *GUIManager) Update() {
	gm.hud.CursorImage = gm.CursorImage
	gm.screens.Update()
}

func (gm *GUIManager) Draw(screen *ebiten.Image) {
	gm.screens.Draw(screen)
}

func (gm *GUIManager) ActiveSidebarTab() string                       { return gm.hud.ActiveSidebarTab() }
func (gm *GUIManager) RefreshPopulationTab(entries []PopulationEntry) { gm.hud.RefreshPopulationTab(entries) }
func (gm *GUIManager) RefreshGoalsTab(lines []string)                  { gm.hud.RefreshGoalsTab(lines) }
func (gm *GUIManager) RefreshCraftQueue(entries []CraftQueueEntry)     { gm.hud.RefreshCraftQueue(entries) }

func (gm *GUIManager) OpenModal(name string)      { gm.hud.OpenModal(name) }
func (gm *GUIManager) CloseModal(name string)     { gm.hud.CloseModal(name) }
func (gm *GUIManager) ModalOpen(name string) bool { return gm.hud.ModalOpen(name) }
func (gm *GUIManager) ToggleModal(name string) {
	if gm.hud.ModalOpen(name) {
		gm.hud.CloseModal(name)
	} else {
		gm.hud.OpenModal(name)
	}
}

func (gm *GUIManager) GetInputFocused() bool            { return gm.hud.GetInputFocused() }
func (gm *GUIManager) GetMouseFocused() bool            { return gm.hud.GetMouseFocused() }
func (gm *GUIManager) WithinModalBounds(x, y int) bool { return gm.hud.WithinModalBounds(x, y) }

func (gm *GUIManager) SetHoveredEntity(entity *ecs.Entity)                        { gm.hud.SetHoveredEntity(entity) }
func (gm *GUIManager) SetHoveredTile(name string, solid, water, air, space bool) { gm.hud.SetHoveredTile(name, solid, water, air, space) }
func (gm *GUIManager) ClearHover()                                                { gm.hud.ClearHover() }
func (gm *GUIManager) UpdateResource(id string, value int)                        { gm.hud.UpdateResource(id, value) }
func (gm *GUIManager) SetSaveNames(names []string)                                { gm.hud.SetSaveNames(names) }
func (gm *GUIManager) ShowColonistModal(colonist *ecs.Entity, storageItems []StorageItemEntry) {
	gm.hud.ShowColonistModal(colonist, storageItems)
}
