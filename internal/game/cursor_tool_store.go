package game

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// storeTool implements the two-phase Store cursor mode: the first click picks an
// item on the ground, the second resolves it to a storage container that accepts
// it. The picked item is highlighted via a Selected component.
type storeTool struct {
	host cursorToolHost
	item *ecs.Entity
}

func (t *storeTool) Mode() gui.CursorModeType { return gui.CursorModeStore }

func (t *storeTool) Enter() {
	t.setItem(nil)
	t.banner()
}

func (t *storeTool) Exit() {
	t.setItem(nil)
	t.host.SetContext("", "", "")
}

// setItem updates the Phase-1 selection and syncs the world-render highlight
// (the Selected component) so the picked item is outlined until resolved.
func (t *storeTool) setItem(item *ecs.Entity) {
	if t.item != nil && t.item != item {
		t.item.RemoveComponent(components.Selected)
	}
	t.item = item
	if item != nil {
		item.AddComponent(&components.SelectedComponent{})
	}
}

// banner shows the static Store hint based on whether an item has been picked.
func (t *storeTool) banner() {
	if t.item == nil {
		t.host.SetContext("Store: Pick an Item", "Click an item on the ground.", "")
		return
	}
	t.host.SetContext("Store: "+entityDisplayLabel(t.item), "Click a storage container that accepts it.", t.item.Blueprint)
}

// Hover drives the per-tile tooltip, branching on whether an item is picked yet.
func (t *storeTool) Hover(tX, tY, tZ int) {
	if t.item == nil {
		// First-click phase: looking for an item on the ground.
		if ent := t.host.Level().GetEntityAt(tX, tY, tZ); ent != nil && ent.HasComponent(rlcomponents.Item) {
			t.host.SetContext("Pick: "+entityDisplayLabel(ent), "Click to choose this item to store.", ent.Blueprint)
			return
		}
		t.banner()
		return
	}

	// Second-click phase: looking for a storage container.
	if destEntity := storageContainerAt(t.host.Level(), tX, tY, tZ); destEntity != nil {
		destName := entityDisplayLabel(destEntity)
		destSc := destEntity.GetComponent(components.Storage).(*components.StorageComponent)
		if !destSc.Accepts(t.item) {
			t.host.SetContext("Won't Accept: "+destName, "Tag filter rejects this item.", t.item.Blueprint)
			return
		}
		t.host.SetContext("Store In: "+destName, "Place "+entityDisplayLabel(t.item)+" here.", t.item.Blueprint)
		return
	}

	t.banner()
}

// Click implements the two-phase order: first click picks an item, second click
// resolves it to a storage destination that accepts it. After queuing the task,
// the mode resets so the player can issue another order without leaving Store.
func (t *storeTool) Click(tX, tY, tZ int) {
	settlement := t.host.Settlement()
	if settlement == nil {
		return
	}

	// Phase 1: pick an item.
	if t.item == nil {
		ent := t.host.Level().GetEntityAt(tX, tY, tZ)
		// A Worker-bearing entity (e.g. a deployed robot) counts as
		// "pickupable" too — that's how the player recalls it back into
		// inventory and then into a locker.
		if ent == nil || (!ent.HasComponent(rlcomponents.Item) && !ent.HasComponent(components.Worker)) {
			message.AddMessage("Pick an item lying on the ground first.")
			return
		}
		t.setItem(ent)
		t.banner()
		message.AddMessage("Picked " + entityDisplayLabel(ent) + ". Click a storage container.")
		return
	}

	// Phase 2: pick a destination container.
	item := t.item
	destEntity := storageContainerAt(t.host.Level(), tX, tY, tZ)
	if destEntity == nil {
		message.AddMessage("That's not a storage container.")
		return
	}
	destSc := destEntity.GetComponent(components.Storage).(*components.StorageComponent)
	if !destSc.Accepts(item) {
		message.AddMessage(entityDisplayLabel(destEntity) + " won't accept " + entityDisplayLabel(item) + ".")
		return
	}

	// Queue the task and reset for another order.
	if !item.HasComponent(rlcomponents.Position) {
		message.AddMessage("Item is no longer on the ground.")
		t.setItem(nil)
		t.banner()
		return
	}
	ipc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	settlement.Tasks.AddTask(&task.Task{
		Action: task_requests.RetrieveAction,
		Data:   task_requests.RetrieveRequest{Item: item, Dest: destEntity},
		X:      ipc.GetX(), Y: ipc.GetY(), Z: ipc.GetZ(),
		Escalated: true,
	})
	message.AddMessage("Queued: store " + entityDisplayLabel(item) + " in " + entityDisplayLabel(destEntity) + ".")
	// Match the other one-shot orders (Sleep/Attack/etc.) — drop back to
	// Default after a successful queue. Exit() clears the item + highlight.
	t.host.RequestMode(gui.CursorModeDefault)
}
