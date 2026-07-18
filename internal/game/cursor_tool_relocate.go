package game

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// relocateTool implements the Relocate cursor mode: after the storage inspector
// picks a material to move, the next click chooses a destination container or
// open tile and queues a relocate task.
type relocateTool struct {
	host    cursorToolHost
	pending *pendingRelocate
}

func (t *relocateTool) Mode() gui.CursorModeType { return gui.CursorModeRelocate }

// Begin is called from the storage inspector when the player chooses to relocate
// a material. It stashes the pending source/blueprint/qty and switches to
// Relocate mode so the next click picks the destination.
func (t *relocateTool) Begin(source *ecs.Entity, blueprint string, qty int) {
	if source == nil || qty <= 0 {
		return
	}
	t.pending = &pendingRelocate{source: source, blueprint: blueprint, qty: qty}
	t.host.RequestMode(gui.CursorModeRelocate)
	message.AddMessage(fmt.Sprintf("Click a storage container or open tile to drop %d %s.", qty, displayMaterialName(blueprint)))
}

func (t *relocateTool) Enter() { t.banner() }

func (t *relocateTool) Exit() {
	t.pending = nil
	t.host.SetContext("", "", "")
}

// Click resolves the destination: a colony-owned container that accepts the
// material queues a container relocate; otherwise a walkable tile queues a
// ground drop. Invalid clicks flash a status and keep the mode active.
func (t *relocateTool) Click(tX, tY, tZ int) {
	pr := t.pending
	if pr == nil || pr.source == nil {
		t.host.RequestMode(gui.CursorModeDefault)
		return
	}
	settlement := t.host.Settlement()
	if settlement == nil {
		return
	}
	dispName := displayMaterialName(pr.blueprint)

	destEntity := storageContainerAt(t.host.Level(), tX, tY, tZ)

	req := task_requests.RelocateRequest{
		Source:    pr.source,
		Blueprint: pr.blueprint,
		Qty:       pr.qty,
		DestX:     tX,
		DestY:     tY,
		DestZ:     tZ,
	}

	if destEntity != nil {
		if destEntity == pr.source {
			message.AddMessage("Source and destination are the same.")
			return
		}
		destSc := destEntity.GetComponent(components.Storage).(*components.StorageComponent)
		if !destSc.AcceptsTags(factory.GetMaterialTags(pr.blueprint)) {
			message.AddMessage(fmt.Sprintf("That container doesn't accept %s.", dispName))
			return
		}
		req.DestEntity = destEntity
		settlement.Tasks.AddTask(&task.Task{
			Action: task_requests.RelocateAction,
			Data:   req,
			X:      tX, Y: tY, Z: tZ,
		})
		message.AddMessage(fmt.Sprintf("Queued: relocate %d %s to %s.", pr.qty, dispName, entityDisplayLabel(destEntity)))
	} else {
		// Ground drop — require a walkable tile at the camera's Z.
		if !t.host.TileWalkable(tX, tY, tZ) {
			message.AddMessage("Can't drop materials there.")
			return
		}
		settlement.Tasks.AddTask(&task.Task{
			Action: task_requests.RelocateAction,
			Data:   req,
			X:      tX, Y: tY, Z: tZ,
		})
		message.AddMessage(fmt.Sprintf("Queued: relocate %d %s to ground.", pr.qty, dispName))
	}
	t.pending = nil
	t.host.RequestMode(gui.CursorModeDefault)
}

// Hover drives the per-tile tooltip: container acceptance, ground-drop, or the
// static banner when the tile isn't a meaningful destination.
func (t *relocateTool) Hover(tX, tY, tZ int) {
	pr := t.pending
	if pr == nil {
		t.host.SetContext("", "", "")
		return
	}

	if destEntity := storageContainerAt(t.host.Level(), tX, tY, tZ); destEntity != nil {
		destName := entityDisplayLabel(destEntity)
		if destEntity == pr.source {
			t.host.SetContext("Source", "Pick another container or open tile.", pr.blueprint)
			return
		}
		destSc := destEntity.GetComponent(components.Storage).(*components.StorageComponent)
		if !destSc.AcceptsTags(factory.GetMaterialTags(pr.blueprint)) {
			t.host.SetContext("Won't Accept: "+destName, "Tag filter rejects this material.", pr.blueprint)
			return
		}
		t.host.SetContext("Drop Into: "+destName, fmt.Sprintf("Move %d here.", pr.qty), pr.blueprint)
		return
	}

	if t.host.TileWalkable(tX, tY, tZ) {
		t.host.SetContext("Drop on Ground", fmt.Sprintf("Place %d here.", pr.qty), pr.blueprint)
		return
	}
	// Unwalkable tile — show the static banner so the player still sees what's
	// in play instead of an empty tooltip.
	t.banner()
}

// banner shows the static Relocate hint when the cursor isn't over a meaningful
// destination tile.
func (t *relocateTool) banner() {
	pr := t.pending
	if pr == nil {
		return
	}
	title := fmt.Sprintf("Relocate: %d × %s", pr.qty, displayMaterialName(pr.blueprint))
	t.host.SetContext(title, "Pick a destination.", pr.blueprint)
}
