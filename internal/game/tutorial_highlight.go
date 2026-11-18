package game

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// highlight is one resolved target: a rectangle, or a ring when radius > 0.
type highlight struct {
	x, y, w, h float32
	radius     float32
}

// highlightTargets resolves the step's highlight list and returns every target
// that is visible right now, so a Travel button and its planet mark together and
// a sub-menu item joins its category once the menu is open. A target that does
// not resolve draws nothing. World targets stand down while a storage inspector
// is open, since their frames would draw across the modal, and every target
// stands down while any other modal covers the screen.
func (b *tutorialBridge) highlightTargets() []highlight {
	step := b.dir.Current()
	if step == nil || b.ended || b.modalCovering() {
		return nil
	}
	var out []highlight
	for _, target := range step.Highlight {
		kind, name, ok := strings.Cut(target, ":")
		if !ok {
			continue
		}
		var hl highlight
		found := false
		switch kind {
		case "hud":
			hl, found = b.hudHighlight(name)
		case "dash":
			hl, found = b.dashHighlight(name)
		case "star":
			hl, found = b.starHighlight(name)
		case "ship":
			hl, found = b.shipHoldHighlight()
		case "inspector":
			hl, found = b.inspectorHighlight(name)
		case "region":
			if b.openInspector() == nil {
				hl, found = b.regionHighlight(name)
			}
		case "entity":
			if b.openInspector() == nil {
				hl, found = b.entityHighlight(name)
			}
		}
		if found {
			out = append(out, hl)
		}
	}
	return out
}

// liveSite is the level this frame was drawn from, if it is the one in orbit
// or the one being viewed (a race run's world, an observer's planet).
func (b *tutorialBridge) liveSite() *MainState {
	if b.lastSite != nil && b.lastCtx == tutorial.ContextSite &&
		(b.lastSite == b.wm.Current() || b.wm.viewedID != "" && b.lastSite.locationID == b.wm.viewedID) {
		return b.lastSite
	}
	return nil
}

func (b *tutorialBridge) hudHighlight(id string) (highlight, bool) {
	s := b.liveSite()
	if s == nil || s.guiManager == nil {
		return highlight{}, false
	}
	x, y, w, h, ok := s.guiManager.ElementBounds(id)
	if !ok || w <= 0 || h <= 0 {
		return highlight{}, false
	}
	return highlight{x: float32(x), y: float32(y), w: float32(w), h: float32(h)}, true
}

// dashWidget maps a highlight name to a dashboard widget.
func (b *tutorialBridge) dashWidget(name string) minui.Element {
	d := b.lastDash
	if d == nil || !strings.HasPrefix(b.lastCtx, "dash:") {
		return nil
	}
	switch name {
	case "returnSiteBtn":
		return d.returnSiteBtn
	case "returnShipBtn":
		return d.returnShipBtn
	case "saveBtn":
		return d.saveBtn
	}
	if p := d.starMapPanel(); p != nil && b.lastCtx == "dash:starmap" {
		switch name {
		case "locList":
			return p.locList
		case "locateBtn":
			return p.locateBtn
		case "travelBtn":
			return p.travelBtn
		case "viewBtn":
			return p.viewBtn
		case "shipList":
			return p.shipList
		case "planList":
			return p.planList
		case "beamDownBtn":
			return p.beamDownBtn
		case "beamUpBtn":
			return p.beamUpBtn
		case "scanBtn":
			return p.scanBtn
		case "leaveBtn":
			return p.leaveBtn
		}
	}
	return nil
}

func (b *tutorialBridge) dashHighlight(name string) (highlight, bool) {
	el := b.dashWidget(name)
	if el == nil || !el.IsVisible() {
		return highlight{}, false
	}
	x, y := el.GetAbsolutePosition()
	r := el.GetBounds()
	if r.Width <= 0 || r.Height <= 0 {
		return highlight{}, false
	}
	return highlight{x: float32(x), y: float32(y), w: float32(r.Width), h: float32(r.Height)}, true
}

// starHighlight rings the tutorial planet or the next destination on the Star
// Map canvas, when that tab is up and the planet is on the canvas.
func (b *tutorialBridge) starHighlight(which string) (highlight, bool) {
	d := b.lastDash
	if d == nil || b.lastCtx != "dash:starmap" {
		return highlight{}, false
	}
	p := d.starMapPanel()
	if p == nil {
		return highlight{}, false
	}
	id := campaign.TutorialLocationID
	if which == tutorial.AtNext {
		id = b.c.Tutorial.NextLocationID
	}
	loc := b.c.Locations[id]
	if loc == nil {
		return highlight{}, false
	}
	sx, sy := p.worldToScreen(loc.X, loc.Y)
	if !p.inCanvas(int(sx), int(sy)) {
		return highlight{}, false
	}
	return highlight{x: sx, y: sy, radius: siteRadius(loc) + 9}, true
}

// modalCovering reports whether a modal other than the storage inspector is up
// on this frame's host: the incident popup, a HUD modal (Esc menu, save, load,
// crafting, research, colonist), the map or the developer console on a site,
// or the scan report on the Star Map. Highlights would draw across it.
func (b *tutorialBridge) modalCovering() bool {
	if b.wm != nil && b.wm.ActiveIncident() != nil {
		return true
	}
	if s := b.lastSite; s != nil {
		if s.guiManager != nil && s.guiManager.AnyModalOpen() {
			return true
		}
		if (s.mapModal != nil && s.mapModal.Visible) || (s.cheatModal != nil && s.cheatModal.Visible) {
			return true
		}
	} else if d := b.lastDash; d != nil && strings.HasPrefix(b.lastCtx, "dash:") {
		if p := d.starMapPanel(); p != nil && p.scanOpen {
			return true
		}
	}
	return false
}

// openInspector returns this frame's host's storage inspector when it is open.
func (b *tutorialBridge) openInspector() *StorageInspectorModal {
	var insp *StorageInspectorModal
	if s := b.lastSite; s != nil {
		insp = s.storageInspector
	} else if d := b.lastDash; d != nil && strings.HasPrefix(b.lastCtx, "dash:") {
		insp = d.storageInspector
	}
	if insp == nil || !insp.Visible {
		return nil
	}
	return insp
}

// inspectorHighlight marks the open storage inspector's Beam button.
func (b *tutorialBridge) inspectorHighlight(name string) (highlight, bool) {
	insp := b.openInspector()
	if insp == nil || insp.actionBtn == nil {
		return highlight{}, false
	}
	var el minui.Element = insp.actionBtn
	if name == "list" && insp.invList != nil {
		el = insp.invList
	}
	x, y := el.GetAbsolutePosition()
	r := el.GetBounds()
	return highlight{x: float32(x), y: float32(y), w: float32(r.Width), h: float32(r.Height)}, true
}

// tileHighlight frames a block of tiles around (wx, wy) on the live level when
// the camera shows it.
func (b *tutorialBridge) tileHighlight(s *MainState, wx, wy, wz, halfTiles int) (highlight, bool) {
	if !s.camera.Contains(wx, wy, wz) {
		return highlight{}, false
	}
	x, y := s.camera.WorldToScreenF(wx-halfTiles, wy-halfTiles)
	size := float32((2*halfTiles + 1))
	return highlight{x: x, y: y, w: size * float32(s.camera.TileW), h: size * float32(s.camera.TileH)}, true
}

func (b *tutorialBridge) regionHighlight(tag string) (highlight, bool) {
	s := b.liveSite()
	if s == nil || s.level == nil {
		return highlight{}, false
	}
	pts := s.level.Regions[tag]
	if len(pts) == 0 {
		return highlight{}, false
	}
	p := pts[0]
	return b.tileHighlight(s, p[0], p[1], p[2], 2)
}

// shipHoldHighlight frames the ship's supply pad while the player is aboard and
// its inspector is closed.
func (b *tutorialBridge) shipHoldHighlight() (highlight, bool) {
	s := b.lastSite
	if s == nil || s != b.wm.shipLevel || b.openInspector() != nil {
		return highlight{}, false
	}
	hold := b.wm.ShipHoldEntity()
	if hold == nil || !hold.HasComponent(rlcomponents.Position) {
		return highlight{}, false
	}
	pc := hold.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	return b.tileHighlight(s, pc.GetX(), pc.GetY(), pc.GetZ(), 0)
}

func (b *tutorialBridge) entityHighlight(blueprint string) (highlight, bool) {
	return b.entityHighlightOn(b.liveSite(), blueprint)
}

// entityHighlightOn is entityHighlight on state s (the vmtest hook passes
// the ship's).
func (b *tutorialBridge) entityHighlightOn(s *MainState, blueprint string) (highlight, bool) {
	if s == nil || s.level == nil {
		return highlight{}, false
	}
	var found *ecs.Entity
	for _, ents := range [][]*ecs.Entity{s.level.Entities, s.level.StaticEntities} {
		for _, e := range ents {
			if e != nil && e.Blueprint == blueprint && e.HasComponent(rlcomponents.Position) && !e.HasComponent(rlcomponents.Dead) {
				pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				if s.camera.Contains(pc.GetX(), pc.GetY(), pc.GetZ()) {
					found = e
					break
				}
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		return highlight{}, false
	}
	pc := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	return b.tileHighlight(s, pc.GetX(), pc.GetY(), pc.GetZ(), 0)
}

// tutMarkerColor is the Star Map marker's colour: magenta, which nothing else
// on the map uses (sites are hue-coloured but drawn solid, selection is gold,
// the cluster boundary green, Home's line amber).
var tutMarkerColor = color.RGBA{255, 70, 255, 255}

// starTarget returns the destination the current step points at on the Star
// Map, or nil.
func (b *tutorialBridge) starTarget() *campaign.Location {
	step := b.dir.Current()
	if step == nil || b.ended || step.Target == "" {
		return nil
	}
	return b.c.Locations[b.targetID(step.Target)]
}

// drawStarMarker makes the step's destination unmistakable on the Star Map:
// two pulsing rings and a label on the planet, the same rings around its row in
// the destination list, and when the planet is off the canvas an arrow at the
// canvas edge pointing at it, so it reads at every zoom and scroll.
func (b *tutorialBridge) drawStarMarker(screen *ebiten.Image) {
	d := b.lastDash
	if d == nil || b.lastCtx != "dash:starmap" {
		return
	}
	p := d.starMapPanel()
	loc := b.starTarget()
	if p == nil || loc == nil {
		return
	}
	pulse := float32(0.5 + 0.5*math.Sin(float64(b.frame)*0.15))
	clr := tutMarkerColor
	clr.A = uint8(150 + 105*pulse)
	label := loc.Name + " - travel here"

	sx, sy := p.worldToScreen(loc.X, loc.Y)
	if p.inCanvas(int(sx), int(sy)) {
		r := siteRadius(loc)
		vector.StrokeCircle(screen, sx, sy, r+8+6*pulse, 3, clr, true)
		vector.StrokeCircle(screen, sx, sy, r+20+10*pulse, 2, clr, true)
		mlge_text.Draw(screen, label, 14, int(sx)+int(r)+16, int(sy)+int(r)+10, tutMarkerColor)
	} else {
		// Clamp to the canvas edge and point the way.
		cx := float32(p.canvasX) + float32(p.canvasW)/2
		cy := float32(p.canvasY) + float32(p.canvasH)/2
		dx, dy := sx-cx, sy-cy
		scale := float32(1)
		if hx := (float32(p.canvasW)/2 - 40) / float32(math.Abs(float64(dx))); dx != 0 && hx < scale {
			scale = hx
		}
		if hy := (float32(p.canvasH)/2 - 40) / float32(math.Abs(float64(dy))); dy != 0 && hy < scale {
			scale = hy
		}
		ex, ey := cx+dx*scale, cy+dy*scale
		vector.StrokeCircle(screen, ex, ey, 14+6*pulse, 3, clr, true)
		vector.StrokeLine(screen, ex, ey, ex+dx/float32(math.Hypot(float64(dx), float64(dy)))*28, ey+dy/float32(math.Hypot(float64(dx), float64(dy)))*28, 3, clr, true)
		mlge_text.Draw(screen, label+" (off screen)", 14, int(ex)-60, int(ey)+22, tutMarkerColor)
	}

	if row, ok := p.rowRect(loc.ID); ok {
		vector.StrokeRect(screen, float32(row.X)-2, float32(row.Y)-2, float32(row.Width)+4, float32(row.Height)+4, 2+pulse, clr, true)
	}
}

// drawHighlight strokes every resolved target with a pulsing outline, and the
// Star Map marker for steps that point at a destination.
func (b *tutorialBridge) drawHighlight(screen *ebiten.Image) {
	if b.modalCovering() {
		return
	}
	b.drawStarMarker(screen)
	pulse := 0.55 + 0.45*math.Sin(float64(b.frame)*0.12)
	clr := color.RGBA{255, 220, 110, uint8(120 + 135*pulse)}
	width := float32(2 + pulse)
	pad := float32(3 + 2*pulse)
	for _, hl := range b.highlightTargets() {
		if hl.radius > 0 {
			vector.StrokeCircle(screen, hl.x, hl.y, hl.radius+float32(2*pulse), width, clr, true)
			continue
		}
		vector.StrokeRect(screen, hl.x-pad, hl.y-pad, hl.w+2*pad, hl.h+2*pad, width, clr, true)
	}
}
