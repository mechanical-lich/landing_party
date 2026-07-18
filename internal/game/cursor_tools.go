package game

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
)

// cursorTool is one modal click interaction (Relocate, Store, Drop, Pickup):
// the player enters it, hovers tiles to preview, clicks a target to resolve,
// and it cleans up when the cursor mode changes away. Extracting these keeps
// each mode's pending state + logic in one cohesive unit instead of smeared
// across MainState, and gives them a uniform Enter/Exit/Click/Hover lifecycle.
type cursorTool interface {
	// Mode is the cursor mode this tool owns.
	Mode() gui.CursorModeType
	// Enter is called when the mode becomes active — set up the banner/state.
	Enter()
	// Exit is called when leaving the mode — drop pending selection and context.
	Exit()
	// Click resolves a left-click on tile (x,y,z) while the mode is active.
	Click(x, y, z int)
	// Hover updates the per-tile context tooltip for tile (x,y,z). Tools without
	// live hover feedback (Drop, Pickup) implement it as a no-op.
	Hover(x, y, z int)
}

// cursorToolHost is the narrow slice of MainState a cursor tool depends on, so
// tools don't reach into the whole state object. MainState satisfies it via the
// adapter methods below.
type cursorToolHost interface {
	// Level is the live level the tool operates on.
	Level() *world.Level
	// Settlement is the player's main settlement, or nil if none exists yet.
	Settlement() *settlement.Settlement
	// SetContext sets the HUD context tooltip (title, body, blueprint icon key).
	// Pass empty strings to clear it.
	SetContext(title, body, blueprint string)
	// OpenColonistModal opens the given colonist's inventory modal.
	OpenColonistModal(colonist *ecs.Entity)
	// TileWalkable reports whether (x,y,z) is a valid ground-drop destination.
	TileWalkable(x, y, z int) bool
	// RequestMode queues a switch to another cursor mode (e.g. back to Default
	// after resolving an order).
	RequestMode(mode gui.CursorModeType)
}

// --- MainState adapters satisfying cursorToolHost ---

func (s *MainState) Level() *world.Level                { return s.level }
func (s *MainState) Settlement() *settlement.Settlement { return s.MainSettlement }
func (s *MainState) SetContext(title, body, bp string) {
	s.guiManager.SetDefaultContext(title, body, bp)
}
func (s *MainState) OpenColonistModal(c *ecs.Entity) { s.openColonistModal(c) }
func (s *MainState) TileWalkable(x, y, z int) bool   { return s.tileWalkable(x, y, z) }

func (s *MainState) RequestMode(mode gui.CursorModeType) {
	event.GetQueuedInstance().QueueEvent(gui.CursorModeChangedEvent{Mode: mode})
}

// initCursorTools builds the modal tools and the mode→tool dispatch map. Called
// once from newMainStateBase.
func (s *MainState) initCursorTools() {
	s.relocateTool = &relocateTool{host: s}
	s.storeTool = &storeTool{host: s}
	s.dropTool = &dropTool{host: s}
	s.pickupTool = &pickupTool{host: s}
	s.cursorTools = map[gui.CursorModeType]cursorTool{
		gui.CursorModeRelocate: s.relocateTool,
		gui.CursorModeStore:    s.storeTool,
		gui.CursorModeDrop:     s.dropTool,
		gui.CursorModePickup:   s.pickupTool,
	}
}

// pendingRelocate carries the source/blueprint/qty chosen in the storage
// inspector, awaiting a destination click in Relocate mode.
type pendingRelocate struct {
	source    *ecs.Entity
	blueprint string
	qty       int
}

// storageContainerAt returns the colony storage container occupying (x,y,z), or
// nil. It checks live entities first, then the level's StaticEntities — the same
// two-pass lookup the Relocate/Store/Drop destination resolvers share.
func storageContainerAt(level *world.Level, x, y, z int) *ecs.Entity {
	if ent := level.GetEntityAt(x, y, z); ent != nil && ent.HasComponent(components.Storage) {
		return ent
	}
	for _, se := range level.StaticEntities {
		if !se.HasComponent(components.Storage) || !se.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := se.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetX() == x && pc.GetY() == y && pc.GetZ() == z {
			return se
		}
	}
	return nil
}
