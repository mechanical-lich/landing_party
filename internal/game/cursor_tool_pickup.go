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

// pickupTool implements the Pickup cursor mode: a colonist chosen from their
// modal walks to a clicked ground item and picks it up into their own bag. The
// task is scoped to that specific colonist rather than the settlement queue.
type pickupTool struct {
	host     cursorToolHost
	colonist *ecs.Entity
}

func (t *pickupTool) Mode() gui.CursorModeType { return gui.CursorModePickup }

// Request is fired from the colonist modal's Pickup button. It captures the
// colonist and switches into Pickup mode so the player can click an item on the
// ground for this specific colonist to fetch.
func (t *pickupTool) Request(colonist *ecs.Entity) {
	if colonist == nil {
		return
	}
	if !colonist.HasComponent(rlcomponents.Inventory) || !colonist.HasComponent(rlcomponents.AIMemory) || !colonist.HasComponent(components.Worker) {
		return
	}
	t.colonist = colonist
	t.host.RequestMode(gui.CursorModePickup)
}

func (t *pickupTool) Enter() { t.banner() }

func (t *pickupTool) Exit() {
	t.colonist = nil
	t.host.SetContext("", "", "")
}

// Hover is a no-op — Pickup shows only the static banner set on Enter.
func (t *pickupTool) Hover(tX, tY, tZ int) {}

// banner shows the static Pickup hint while the player is choosing the item.
func (t *pickupTool) banner() {
	if t.colonist == nil {
		t.host.SetContext("Pickup", "Open a colonist's detail view and start there.", "")
		return
	}
	t.host.SetContext("Pickup ("+entityDisplayLabel(t.colonist)+")", "Click an item on the ground.", "")
}

// Click resolves the target-item click. The target must be an entity with Item
// (or a deployed Worker — same rule as Store-mode recall). It assigns a Pickup
// task scoped to the selected colonist via Worker.CurrentTask so the settlement
// queue doesn't hand it to whichever hauler picks it up first.
func (t *pickupTool) Click(tX, tY, tZ int) {
	if t.colonist == nil {
		message.AddMessage("Open a colonist's detail view first.")
		return
	}
	target := t.host.Level().GetEntityAt(tX, tY, tZ)
	if target == nil || (!target.HasComponent(rlcomponents.Item) && !target.HasComponent(components.Worker)) {
		message.AddMessage("Pick an item lying on the ground.")
		return
	}
	if target == t.colonist {
		return
	}
	colonist := t.colonist
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	aiMemory := colonist.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.Stop()
	}
	// Use PickupAction (not Retrieve) — Retrieve walks to a storage container
	// after pickup; the player asked for the item to go into the colonist's
	// own bag and stay there.
	tsk := &task.Task{
		Action: task_requests.PickupAction,
		Data:   target,
		X:      tX, Y: tY, Z: tZ,
		Escalated: true,
	}
	tsk.Start()
	wc.CurrentTask = tsk
	aiMemory.State = "task"
	message.AddMessage(entityDisplayLabel(colonist) + ": pick up " + entityDisplayLabel(target) + ".")
	t.host.RequestMode(gui.CursorModeDefault)
}
