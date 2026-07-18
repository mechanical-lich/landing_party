package game

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// dropTool implements the Drop cursor mode: an item chosen from a colonist's
// inventory modal is dropped onto a ground tile or into a storage container the
// destination click resolves. The (colonist, item) pair is captured up front.
type dropTool struct {
	host     cursorToolHost
	colonist *ecs.Entity
	item     *ecs.Entity
}

func (t *dropTool) Mode() gui.CursorModeType { return gui.CursorModeDrop }

// Request is fired from the colonist inventory modal's per-item Drop button. It
// captures (colonist, item) and switches into Drop mode so the player can click
// a ground tile or storage container as the destination.
func (t *dropTool) Request(colonist, item *ecs.Entity) {
	if colonist == nil || item == nil {
		return
	}
	if !colonist.HasComponent(rlcomponents.Inventory) || !colonist.HasComponent(rlcomponents.AIMemory) || !colonist.HasComponent(components.Worker) {
		return
	}
	t.colonist = colonist
	t.item = item
	t.host.RequestMode(gui.CursorModeDrop)
}

func (t *dropTool) Enter() { t.banner() }

func (t *dropTool) Exit() {
	t.colonist = nil
	t.item = nil
	t.host.SetContext("", "", "")
}

// Hover is a no-op — Drop shows only the static banner set on Enter.
func (t *dropTool) Hover(tX, tY, tZ int) {}

// banner shows the static Drop hint based on whether an item has already been
// picked from a colonist's inventory.
func (t *dropTool) banner() {
	if t.item == nil {
		t.host.SetContext("Drop", "Open a colonist and pick an item from their bag.", "")
		return
	}
	t.host.SetContext("Drop "+entityDisplayLabel(t.item),
		"Click a ground tile or storage container.", t.item.Blueprint)
}

// Click resolves the destination. Without a pending item, a click on a colonist
// with a non-empty bag opens their inventory to choose what to drop; otherwise
// the destination is a storage container at the tile or the bare ground tile.
func (t *dropTool) Click(tX, tY, tZ int) {
	// Phase 1: no item picked yet. Treat a click on a colonist with a
	// non-empty bag as "open this colonist's inventory so they can choose
	// what to drop" — same gesture used by the Orders → Drop entry point.
	if t.item == nil || t.colonist == nil {
		ent := t.host.Level().GetEntityAt(tX, tY, tZ)
		if ent != nil && ent.HasComponent(components.Worker) && ent.HasComponent(rlcomponents.Inventory) {
			inv := ent.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			if len(inv.Bag) == 0 {
				message.AddMessage(entityDisplayLabel(ent) + " isn't carrying anything.")
				return
			}
			t.host.OpenColonistModal(ent)
			return
		}
		message.AddMessage("Open a colonist and pick an item to drop first.")
		return
	}
	colonist := t.colonist
	item := t.item
	inv := colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	// Guard against the item being lost between picking and clicking (e.g.
	// the colonist was killed, or the item was equipped from the modal).
	stillHas := false
	for _, b := range inv.Bag {
		if b == item {
			stillHas = true
			break
		}
	}
	if !stillHas {
		message.AddMessage(entityDisplayLabel(colonist) + " no longer carries that item.")
		t.host.RequestMode(gui.CursorModeDefault)
		return
	}

	// Resolve destination: storage container at the tile, else bare tile.
	destEntity := storageContainerAt(t.host.Level(), tX, tY, tZ)
	if destEntity != nil {
		destSc := destEntity.GetComponent(components.Storage).(*components.StorageComponent)
		if !destSc.Accepts(item) {
			message.AddMessage(entityDisplayLabel(destEntity) + " won't accept " + entityDisplayLabel(item) + ".")
			return
		}
	} else {
		// Ground destination needs a walkable tile (matches the rest of the
		// click handlers — anything else is just a misclick).
		ti := t.host.Level().GetTileAt(tX, tY, tZ)
		if ti == nil {
			return
		}
		tile := ti.(*world.Tile)
		if tile.IsSolid() || tile.IsWater() {
			message.AddMessage("Can't drop there.")
			return
		}
	}

	aiMemory := colonist.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
	}
	wc.DropOffItem = item
	wc.DropOffDestEntity = destEntity
	if destEntity != nil {
		pc := destEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		wc.DropOffX, wc.DropOffY, wc.DropOffZ = pc.GetX(), pc.GetY(), pc.GetZ()
		aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ = pc.GetX(), pc.GetY(), pc.GetZ()
		message.AddMessage("Queued: drop " + entityDisplayLabel(item) + " in " + entityDisplayLabel(destEntity) + ".")
	} else {
		wc.DropOffX, wc.DropOffY, wc.DropOffZ = tX, tY, tZ
		aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ = tX, tY, tZ
		message.AddMessage("Queued: drop " + entityDisplayLabel(item) + ".")
	}
	aiMemory.State = "dropoff"
	t.host.RequestMode(gui.CursorModeDefault)
}
