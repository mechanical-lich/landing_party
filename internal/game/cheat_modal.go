package game

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/ui/minui"
)

const (
	cheatModalW = 460
	cheatModalH = 160
)

// CheatModal is a developer console triggered by Shift+ESC. It mirrors the
// spaceplant cheat console but is adapted for landing_party's free camera
// (there is no player entity): find/mv work relative to the camera, and
// tp can target an entity by blueprint or by individual colonist name.
type CheatModal struct {
	Visible bool
	ms      *MainState
	modal   *minui.Modal
	input   *minui.TextInput
}

func newCheatModal(ms *MainState) *CheatModal {
	cm := &CheatModal{ms: ms}
	cm.rebuildModal()
	return cm
}

// Open resets and shows the modal with a fresh TextInput (avoids stale cursorPos).
func (cm *CheatModal) Open() {
	cm.rebuildModal()
	cm.Visible = true
}

func (cm *CheatModal) rebuildModal() {
	cfg := config.Global()
	mx := (cfg.ScreenWidth - cheatModalW) / 2
	my := (cfg.ScreenHeight - cheatModalH) / 2

	cm.modal = minui.NewModal("cheat_modal", "Developer Console", cheatModalW, cheatModalH)
	cm.modal.SetPosition(mx, my)
	cm.modal.Closeable = false

	cm.input = minui.NewTextInput("cheat_input", "find <name>  |  mv <x> <y> <z>  |  tp <name> <x> <y> <z>  |  give <resource> <amount>  |  research <tech_id>")
	cm.input.SetPosition(10, 20)
	cm.input.SetSize(cheatModalW-20, 30)
	cm.modal.AddChild(cm.input)

	okBtn := minui.NewButton("cheat_ok", "OK")
	okBtn.SetPosition((cheatModalW-180)/2-5, cheatModalH-75)
	okBtn.SetSize(80, 30)
	okBtn.OnClick = func() {
		cm.runCommand(cm.input.Text)
		cm.Visible = false
	}
	cm.modal.AddChild(okBtn)

	cancelBtn := minui.NewButton("cheat_cancel", "Cancel")
	cancelBtn.SetPosition((cheatModalW-180)/2+85, cheatModalH-75)
	cancelBtn.SetSize(80, 30)
	cancelBtn.OnClick = func() {
		cm.Visible = false
	}
	cm.modal.AddChild(cancelBtn)
}

// entityName returns the lowercased individual name of an entity (from its
// DescriptionComponent) if it has one, else "".
func entityName(e *ecs.Entity) string {
	if e == nil || !e.HasComponent(rlcomponents.Description) {
		return ""
	}
	dc, ok := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
	if !ok {
		return ""
	}
	return strings.ToLower(dc.Name)
}

// matchByName reports whether an entity matches a query by either its
// blueprint (type, e.g. "colonist") or its individual name (e.g. "jane"),
// case-insensitive substring.
func matchByName(e *ecs.Entity, query string) bool {
	if e == nil {
		return false
	}
	if strings.Contains(strings.ToLower(e.Blueprint), query) {
		return true
	}
	if n := entityName(e); n != "" && strings.Contains(n, query) {
		return true
	}
	return false
}

// cameraCenter returns the world tile the camera is centred on.
func (cm *CheatModal) cameraCenter() (int, int, int) {
	cam := cm.ms.camera
	return cam.X + cam.ViewW()/2, cam.Y + cam.ViewH()/2, cam.Z
}

// centerCameraOn places the camera so the given tile sits in the middle of
// the viewport, matching the small-map / map-modal double-click behaviour.
func (cm *CheatModal) centerCameraOn(x, y, z int) {
	cm.ms.camera.CenterOn(x, y, z)
}

func (cm *CheatModal) inBounds(x, y, z int) bool {
	l := cm.ms.level
	if l == nil {
		return false
	}
	return x >= 0 && x < l.GetWidth() &&
		y >= 0 && y < l.GetHeight() &&
		z >= 0 && z < l.GetDepth()
}

func entityPos(e *ecs.Entity) (*rlcomponents.PositionComponent, bool) {
	if e == nil || !e.HasComponent(rlcomponents.Position) {
		return nil, false
	}
	pc, ok := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	return pc, ok
}

func (cm *CheatModal) runCommand(raw string) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return
	}
	switch strings.ToLower(parts[0]) {
	case "find":
		if len(parts) < 2 {
			message.PostMessage("cheat", "usage: find <name>")
			return
		}
		target := strings.ToLower(strings.Join(parts[1:], " "))
		cx, cy, cz := cm.cameraCenter()

		bestDist := -1
		bestX, bestY, bestZ := 0, 0, 0
		var bestE *ecs.Entity
		for _, e := range append(cm.ms.level.Entities, cm.ms.level.StaticEntities...) {
			if !matchByName(e, target) {
				continue
			}
			pc, ok := entityPos(e)
			if !ok {
				continue
			}
			dx := pc.GetX() - cx
			dy := pc.GetY() - cy
			dz := (pc.GetZ() - cz) * 50
			dist := dx*dx + dy*dy + dz*dz
			if bestDist < 0 || dist < bestDist {
				bestDist = dist
				bestX, bestY, bestZ = pc.GetX(), pc.GetY(), pc.GetZ()
				bestE = e
			}
		}
		if bestDist < 0 {
			message.PostMessage("cheat", fmt.Sprintf("no %q found", target))
		} else {
			label := bestE.Blueprint
			if n := entityName(bestE); n != "" {
				label = fmt.Sprintf("%s (%s)", bestE.Blueprint, n)
			}
			message.PostMessage("cheat", fmt.Sprintf("nearest %s at %d,%d,%d", label, bestX, bestY, bestZ))
		}

	case "mv", "move":
		if len(parts) < 4 {
			message.PostMessage("cheat", "usage: mv <x> <y> <z>")
			return
		}
		x, ex := strconv.Atoi(parts[1])
		y, ey := strconv.Atoi(parts[2])
		z, ez := strconv.Atoi(parts[3])
		if ex != nil || ey != nil || ez != nil {
			message.PostMessage("cheat", "mv: invalid coordinates")
			return
		}
		if !cm.inBounds(x, y, z) {
			message.PostMessage("cheat", "mv: out of bounds")
			return
		}
		cm.centerCameraOn(x, y, z)
		message.PostMessage("cheat", fmt.Sprintf("camera -> %d,%d,%d", x, y, z))

	case "tp", "teleport":
		if len(parts) < 5 {
			message.PostMessage("cheat", "usage: tp <name> <x> <y> <z>")
			return
		}
		z, ez := strconv.Atoi(parts[len(parts)-1])
		y, ey := strconv.Atoi(parts[len(parts)-2])
		x, ex := strconv.Atoi(parts[len(parts)-3])
		if ex != nil || ey != nil || ez != nil {
			message.PostMessage("cheat", "tp: invalid coordinates")
			return
		}
		if !cm.inBounds(x, y, z) {
			message.PostMessage("cheat", "tp: out of bounds")
			return
		}
		target := strings.ToLower(strings.Join(parts[1:len(parts)-3], " "))

		// Pick the entity matching the name that is nearest the camera, so an
		// ambiguous query (e.g. "colonist") resolves predictably.
		cx, cy, cz := cm.cameraCenter()
		bestDist := -1
		var best *ecs.Entity
		for _, e := range cm.ms.level.Entities {
			if !matchByName(e, target) {
				continue
			}
			pc, ok := entityPos(e)
			if !ok {
				continue
			}
			dx := pc.GetX() - cx
			dy := pc.GetY() - cy
			dz := (pc.GetZ() - cz) * 50
			dist := dx*dx + dy*dy + dz*dz
			if bestDist < 0 || dist < bestDist {
				bestDist = dist
				best = e
			}
		}
		if best == nil {
			message.PostMessage("cheat", fmt.Sprintf("no %q found", target))
			return
		}
		pc, _ := entityPos(best)
		pc.SetPosition(x, y, z)
		label := best.Blueprint
		if n := entityName(best); n != "" {
			label = fmt.Sprintf("%s (%s)", best.Blueprint, n)
		}
		message.PostMessage("cheat", fmt.Sprintf("teleported %s to %d,%d,%d", label, x, y, z))

	case "give":
		if len(parts) < 3 {
			message.PostMessage("cheat", "usage: give <resource> <amount>")
			return
		}
		blueprint := strings.ToLower(parts[1])
		amount, err := strconv.Atoi(parts[2])
		if err != nil || amount <= 0 {
			message.PostMessage("cheat", "give: amount must be a positive integer")
			return
		}
		item, err := factory.Create(blueprint, 0, 0, 0)
		if err != nil {
			message.PostMessage("cheat", fmt.Sprintf("give: unknown blueprint %q", blueprint))
			return
		}
		if item.HasComponent(components.Material) {
			item.GetComponent(components.Material).(*components.MaterialComponent).Quantity = amount
		}
		// Find the player's settlement name from any colonist on the level.
		settlementName := ""
		for _, e := range cm.ms.level.Entities {
			if e.HasComponent(components.Settlement) && e.HasComponent(components.Worker) {
				settlementName = e.GetComponent(components.Settlement).(*components.SettlementComponent).Name
				break
			}
		}
		// Deposit into the first storage container owned by the player's settlement.
		for _, e := range cm.ms.level.Entities {
			if !e.HasComponent(components.Storage) {
				continue
			}
			st := e.GetComponent(components.Storage).(*components.StorageComponent)
			if settlementName != "" && st.OwnedBy != settlementName {
				continue
			}
			st.AddItem(item)
			message.PostMessage("cheat", fmt.Sprintf("gave %d %s to storage", amount, blueprint))
			return
		}
		// No storage — drop at camera center so haulers can collect it.
		cx, cy, cz := cm.cameraCenter()
		if pc, ok := entityPos(item); ok {
			pc.SetPosition(cx, cy, cz)
		}
		cm.ms.level.AddEntity(item)
		message.PostMessage("cheat", fmt.Sprintf("dropped %d %s at camera (no storage found)", amount, blueprint))

	case "research":
		if len(parts) < 2 {
			message.PostMessage("cheat", "usage: research <tech_id>")
			return
		}
		key := strings.ToLower(parts[1])
		if cm.ms.campaign == nil {
			message.PostMessage("cheat", "research: no active campaign")
			return
		}
		if _, ok := research.GetTech(key); !ok {
			message.PostMessage("cheat", fmt.Sprintf("research: unknown tech %q", key))
			return
		}
		if cm.ms.campaign.HasTech(key) {
			message.PostMessage("cheat", fmt.Sprintf("research: %s already known", key))
			return
		}
		cm.ms.campaign.UnlockTech(key)
		message.PostMessage("cheat", fmt.Sprintf("researched %s", key))

	default:
		message.PostMessage("cheat", "unknown command: "+parts[0])
	}
}

func (cm *CheatModal) Update() {
	if !cm.Visible {
		return
	}
	cm.modal.Update()
}

func (cm *CheatModal) Draw(screen *ebiten.Image) {
	if !cm.Visible {
		return
	}
	cm.modal.Draw(screen)
}
