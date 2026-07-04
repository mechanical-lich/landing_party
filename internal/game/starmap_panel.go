package game

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// starMapPanel is the visual star map: discovered locations are drawn as
// procedural icons at their X/Y coordinates on a pan/zoom canvas. Clicking (or
// list-selecting) a location shows its fuel cost and enables Travel; once in
// orbit the crew lists let you beam colonists between ship and surface.
type starMapPanel struct {
	campaign *campaign.Campaign
	wm       *WorldManager

	// Canvas rect (content region above the bottom control strip).
	canvasX, canvasY, canvasW, canvasH int

	// World-space camera: (camX,camY) is the world point at the canvas centre;
	// zoom is screen pixels per world unit.
	camX, camY, zoom float64
	fitZoom          float64 // base fit-to-all zoom, reused for Locate framing
	inited           bool

	// drag state
	dragging         bool
	dragMoved        bool
	lastMX, lastMY   int
	pressMX, pressMY int

	locList  *minui.ListBox
	locIDs   []string
	shipList *minui.ListBox
	planList *minui.ListBox
	shipEnts []*ecs.Entity
	planEnts []*ecs.Entity

	locateBtn   *minui.Button
	travelBtn   *minui.Button
	beamDownBtn *minui.Button
	beamUpBtn   *minui.Button

	status  string
	pending state.StateInterface
}

func newStarMapPanel(c *campaign.Campaign, wm *WorldManager) *starMapPanel {
	p := &starMapPanel{campaign: c, wm: wm, zoom: 1}
	cfg := config.Global()
	sw, sh := cfg.ScreenWidth, cfg.ScreenHeight

	p.canvasX = 0
	p.canvasY = dashContentTop
	p.canvasW = sw
	p.canvasH = sh - dashContentTop - 210

	listY := sh - 188
	btnY := sh - 64

	p.locList = minui.NewListBox("sm_locs", nil)
	p.locList.SetBounds(minui.Rect{X: 20, Y: listY, Width: 300, Height: 116})
	p.locList.Layout()
	p.locList.OnSelect = func(idx int, _ string) {} // selection read via SelectedIndex

	p.shipList = minui.NewListBox("sm_ship", nil)
	p.shipList.SetBounds(minui.Rect{X: 340, Y: listY, Width: 250, Height: 116})
	p.shipList.Layout()

	p.planList = minui.NewListBox("sm_plan", nil)
	p.planList.SetBounds(minui.Rect{X: 610, Y: listY, Width: 250, Height: 116})
	p.planList.Layout()

	p.locateBtn = minui.NewButton("sm_locate", "Locate")
	p.locateBtn.SetPosition(20, btnY)
	p.locateBtn.SetSize(140, 30)
	p.locateBtn.OnClick = p.locate

	p.travelBtn = minui.NewButton("sm_travel", "Travel Here")
	p.travelBtn.SetPosition(170, btnY)
	p.travelBtn.SetSize(150, 30)
	p.travelBtn.OnClick = p.travel

	p.beamDownBtn = minui.NewButton("sm_beam_down", "Beam Down  v")
	p.beamDownBtn.SetPosition(340, btnY)
	p.beamDownBtn.SetSize(250, 30)
	p.beamDownBtn.OnClick = p.beamDown

	p.beamUpBtn = minui.NewButton("sm_beam_up", "Beam Up  ^")
	p.beamUpBtn.SetPosition(610, btnY)
	p.beamUpBtn.SetSize(250, 30)
	p.beamUpBtn.OnClick = p.beamUp

	p.refreshLocations()
	p.refreshCrew()
	return p
}

// ---- data ----

func (p *starMapPanel) refreshLocations() {
	p.locIDs = p.locIDs[:0]
	var labels []string
	sel := p.locList.SelectedIndex
	for _, loc := range p.campaign.DiscoveredLocations() {
		p.locIDs = append(p.locIDs, loc.ID)
		tag := ""
		if loc.ID == p.campaign.CurrentLocationID && p.wm.current != nil {
			tag = "  [in orbit]"
		} else if loc.Visited {
			tag = "  [established]"
		}
		scenarioName := ""
		if config.Global().DebugShowScenarioID {
			if s := scenario.ByID(loc.ScenarioID); s != nil {
				scenarioName = " · " + s.Name
			}
		}
		labels = append(labels, fmt.Sprintf("%s (%s%s) — fuel %d%s", loc.Name, loc.Kind, scenarioName, p.campaign.FuelCost(loc.ID), tag))
	}
	p.locList.SetItems(labels)
	if sel >= 0 && sel < len(labels) {
		p.locList.SelectedIndex = sel
	} else if len(labels) > 0 {
		p.locList.SelectedIndex = 0
	}
}

func (p *starMapPanel) refreshCrew() {
	p.shipEnts = p.wm.ShipColonists()
	ship := make([]string, 0, len(p.shipEnts))
	for i, e := range p.shipEnts {
		ship = append(ship, fmt.Sprintf("%d. %s", i+1, entityDisplayName(e)))
	}
	if len(ship) == 0 {
		ship = []string{"(no colonists aboard)"}
	}
	p.shipList.SetItems(ship)

	p.planEnts = p.wm.PlanetColonists()
	plan := make([]string, 0, len(p.planEnts))
	for i, e := range p.planEnts {
		plan = append(plan, fmt.Sprintf("%d. %s", i+1, entityDisplayName(e)))
	}
	if p.wm.current == nil {
		plan = []string{"(not in orbit — Travel first)"}
	} else if len(plan) == 0 {
		plan = []string{"(nobody planetside)"}
	}
	p.planList.SetItems(plan)
}

func (p *starMapPanel) selectedLocation() *campaign.Location {
	i := p.locList.SelectedIndex
	if i < 0 || i >= len(p.locIDs) {
		return nil
	}
	return p.campaign.Locations[p.locIDs[i]]
}

func (p *starMapPanel) fuelAvailable() int {
	return storage.CountResource(
		storage.ShipProvider{Level: p.wm.ShipLevel()},
		[]string{p.wm.shipSettlementName()},
		"fuel",
	)
}

// ---- actions ----

func (p *starMapPanel) locate() {
	loc := p.selectedLocation()
	if loc == nil {
		return
	}
	p.camX, p.camY = loc.X, loc.Y
	// Zoom in to frame the location (never zoom back out if already closer).
	if target := clampZoom(p.fitZoom * locateZoomFactor); target > p.zoom {
		p.zoom = target
	}
}

// locateZoomFactor is how far past the fit-to-all zoom the Locate button snaps
// in on a selected location.
const locateZoomFactor = 7.0

// homeLocation returns the campaign's Home destination, or nil if absent.
func (p *starMapPanel) homeLocation() *campaign.Location {
	for _, l := range p.campaign.Locations {
		if l.Kind == campaign.HomeKind {
			return l
		}
	}
	return nil
}

func (p *starMapPanel) travel() {
	loc := p.selectedLocation()
	if loc == nil {
		return
	}
	if loc.ID == p.campaign.CurrentLocationID && p.wm.current != nil {
		p.status = "Already in orbit here. Beam crew, then use To Site."
		return
	}
	moving := loc.ID != p.campaign.CurrentLocationID
	cost := p.campaign.FuelCost(loc.ID)
	if moving && p.fuelAvailable() < cost {
		p.status = fmt.Sprintf("Not enough fuel: have %d, need %d.", p.fuelAvailable(), cost)
		return
	}
	if err := p.wm.Travel(loc.ID); err != nil {
		p.status = "Travel failed: " + err.Error()
		return
	}
	if moving && cost > 0 {
		storage.Deduct(
			storage.ShipProvider{Level: p.wm.ShipLevel()},
			[]string{p.wm.shipSettlementName()},
			map[string]int{"fuel": cost},
		)
	}
	if p.campaign.Won {
		p.pending = NewCampaignEndState(true, fmt.Sprintf("The colonists are home at last. (Day %d)", p.campaign.Day))
		return
	}
	p.status = fmt.Sprintf("In orbit over %s. Beam crew down, then use To Site.", loc.Name)
	p.refreshLocations()
	p.refreshCrew()
}

func (p *starMapPanel) beamDown() {
	if p.wm.current == nil {
		p.status = "Travel to a location before beaming down."
		return
	}
	idx := p.shipList.SelectedIndex
	if idx < 0 || idx >= len(p.shipEnts) {
		p.status = "Select a colonist on the ship."
		return
	}
	if err := p.wm.BeamDown(p.shipEnts[idx]); err != nil {
		p.status = err.Error()
		return
	}
	p.status = "Colonist beamed down."
	p.refreshCrew()
}

func (p *starMapPanel) beamUp() {
	idx := p.planList.SelectedIndex
	if idx < 0 || idx >= len(p.planEnts) {
		p.status = "Select a colonist planetside."
		return
	}
	if err := p.wm.BeamUp(p.planEnts[idx]); err != nil {
		p.status = err.Error()
		return
	}
	p.status = "Colonist beamed up to the ship."
	p.refreshCrew()
}

// ---- panel interface ----

func (p *starMapPanel) Enter() {
	p.refreshLocations()
	p.refreshCrew()
	if !p.inited {
		p.fitView()
		p.inited = true
	}
}

func (p *starMapPanel) Update() {
	p.handleCanvasInput()
	p.locList.Update()
	p.locateBtn.Update()
	p.travelBtn.Update()

	inOrbit := p.wm.current != nil
	p.beamDownBtn.SetEnabled(inOrbit)
	p.beamUpBtn.SetEnabled(inOrbit)
	p.shipList.Update()
	p.planList.Update()
	p.beamDownBtn.Update()
	p.beamUpBtn.Update()
}

func (p *starMapPanel) Transition() state.StateInterface {
	t := p.pending
	p.pending = nil
	return t
}

// ---- canvas ----

func (p *starMapPanel) fitView() {
	locs := p.campaign.DiscoveredLocations()
	if len(locs) == 0 {
		p.camX, p.camY, p.zoom = 0, 0, 1
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, l := range locs {
		minX, maxX = math.Min(minX, l.X), math.Max(maxX, l.X)
		minY, maxY = math.Min(minY, l.Y), math.Max(maxY, l.Y)
	}
	if cur := p.campaign.CurrentLocation(); cur != nil {
		p.camX, p.camY = cur.X, cur.Y
	} else {
		p.camX, p.camY = (minX+maxX)/2, (minY+maxY)/2
	}
	spanX, spanY := math.Max(maxX-minX, 1), math.Max(maxY-minY, 1)
	zx := float64(p.canvasW-120) / spanX
	zy := float64(p.canvasH-120) / spanY
	p.fitZoom = math.Min(zx, zy)
	// Zoom in past a pure fit-all so the central planets read large by default;
	// outer locations are a short pan away.
	p.zoom = clampZoom(p.fitZoom * defaultZoomFactor)
}

// defaultZoomFactor tightens the initial fit-to-all view so the map opens
// closer on the central cluster rather than showing the whole extent.
const defaultZoomFactor = 1.75

func clampZoom(z float64) float64 {
	if z < 0.05 {
		return 0.05
	}
	if z > 16 {
		return 16
	}
	return z
}

func (p *starMapPanel) worldToScreen(wx, wy float64) (float32, float32) {
	ccx := float64(p.canvasX) + float64(p.canvasW)/2
	ccy := float64(p.canvasY) + float64(p.canvasH)/2
	return float32(ccx + (wx-p.camX)*p.zoom), float32(ccy + (wy-p.camY)*p.zoom)
}

func (p *starMapPanel) inCanvas(x, y int) bool {
	return x >= p.canvasX && x < p.canvasX+p.canvasW && y >= p.canvasY && y < p.canvasY+p.canvasH
}

func (p *starMapPanel) handleCanvasInput() {
	mx, my := ebiten.CursorPosition()
	if p.inCanvas(mx, my) {
		if _, wy := ebiten.Wheel(); wy != 0 {
			ccx := float64(p.canvasX) + float64(p.canvasW)/2
			ccy := float64(p.canvasY) + float64(p.canvasH)/2
			// World point under the cursor, kept fixed across the zoom.
			wxAt := p.camX + (float64(mx)-ccx)/p.zoom
			wyAt := p.camY + (float64(my)-ccy)/p.zoom
			if wy > 0 {
				p.zoom = clampZoom(p.zoom * 1.15)
			} else {
				p.zoom = clampZoom(p.zoom / 1.15)
			}
			p.camX = wxAt - (float64(mx)-ccx)/p.zoom
			p.camY = wyAt - (float64(my)-ccy)/p.zoom
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && p.inCanvas(mx, my) {
		p.dragging = true
		p.dragMoved = false
		p.lastMX, p.lastMY = mx, my
		p.pressMX, p.pressMY = mx, my
	}
	if p.dragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if dx, dy := mx-p.lastMX, my-p.lastMY; dx != 0 || dy != 0 {
				p.camX -= float64(dx) / p.zoom
				p.camY -= float64(dy) / p.zoom
				p.lastMX, p.lastMY = mx, my
				if abs(mx-p.pressMX)+abs(my-p.pressMY) > 4 {
					p.dragMoved = true
				}
			}
		} else {
			p.dragging = false
			if !p.dragMoved {
				p.hitSelect(mx, my)
			}
		}
	}
}

func (p *starMapPanel) hitSelect(mx, my int) {
	best, bestDist := -1, math.Inf(1)
	for i, id := range p.locIDs {
		loc := p.campaign.Locations[id]
		if loc == nil {
			continue
		}
		sx, sy := p.worldToScreen(loc.X, loc.Y)
		d := math.Hypot(float64(sx)-float64(mx), float64(sy)-float64(my))
		hitR := float64(kindRadius(loc.Kind)) + 8
		if d <= hitR && d < bestDist {
			best, bestDist = i, d
		}
	}
	if best >= 0 {
		p.locList.SelectedIndex = best
	}
}

func (p *starMapPanel) Draw(screen *ebiten.Image) {
	// Canvas background + clip.
	canvasRect := minui.Rect{X: p.canvasX, Y: p.canvasY, Width: p.canvasW, Height: p.canvasH}
	minui.DrawRect(screen, canvasRect, color.RGBA{10, 12, 22, 255})
	minui.DrawRectStroke(screen, canvasRect, 1, color.RGBA{40, 55, 80, 255})
	canvas := screen.SubImage(image.Rect(p.canvasX, p.canvasY, p.canvasX+p.canvasW, p.canvasY+p.canvasH)).(*ebiten.Image)

	sel := p.selectedLocation()
	cur := p.campaign.CurrentLocation()

	// Fuel-range ring around the current position (1 fuel ≈ 1 world unit).
	if cur != nil {
		cx, cy := p.worldToScreen(cur.X, cur.Y)
		r := float32(p.fuelAvailable()) * float32(p.zoom)
		if r > 2 {
			vector.StrokeCircle(canvas, cx, cy, r, 1, color.RGBA{60, 120, 90, 160}, true)
		}
		// Dashed line to Home — the goal of the run.
		if home := p.homeLocation(); home != nil && home != cur {
			hx, hy := p.worldToScreen(home.X, home.Y)
			drawDashedLine(canvas, cx, cy, hx, hy, 9, 6, 1.5, color.RGBA{230, 200, 120, 170})
		}
		// Path line from current to the selected destination.
		if sel != nil && sel != cur {
			sx, sy := p.worldToScreen(sel.X, sel.Y)
			vector.StrokeLine(canvas, cx, cy, sx, sy, 1, color.RGBA{80, 110, 150, 160}, true)
		}
	}

	for _, id := range p.locIDs {
		loc := p.campaign.Locations[id]
		if loc == nil {
			continue
		}
		sx, sy := p.worldToScreen(loc.X, loc.Y)
		r := kindRadius(loc.Kind)
		vector.DrawFilledCircle(canvas, sx, sy, r, kindColor(loc.Kind), true)
		if loc == cur {
			vector.StrokeCircle(canvas, sx, sy, r+5, 2, color.RGBA{120, 220, 160, 255}, true)
		}
		if loc == sel {
			vector.StrokeCircle(canvas, sx, sy, r+3, 2, color.RGBA{255, 230, 120, 255}, true)
		}
		mlge_text.Draw(canvas, loc.Name, 11, int(sx)+int(r)+4, int(sy)-7, color.RGBA{190, 205, 225, 255})
	}

	// Bottom control strip.
	sh := config.Global().ScreenHeight
	lblY := sh - 204
	lbl := color.RGBA{160, 185, 215, 255}
	mlge_text.Draw(screen, "Destinations", 14, 20, lblY, lbl)
	mlge_text.Draw(screen, "On Ship", 14, 340, lblY, lbl)
	planTitle := "Planetside"
	if cur != nil && p.wm.current != nil {
		planTitle = "On " + cur.Name
	}
	mlge_text.Draw(screen, planTitle, 14, 610, lblY, lbl)

	p.locList.Draw(screen)
	p.shipList.Draw(screen)
	p.planList.Draw(screen)
	p.locateBtn.Draw(screen)
	p.travelBtn.Draw(screen)
	p.beamDownBtn.Draw(screen)
	p.beamUpBtn.Draw(screen)

	// Selected-destination info (right of the crew columns).
	infoX := 880
	infoW := config.Global().ScreenWidth - infoX - 24
	if infoW < 220 {
		infoW = 220
	}
	if sel != nil {
		mlge_text.Draw(screen, sel.Name, 16, infoX, lblY, color.RGBA{220, 230, 255, 255})
		line := fmt.Sprintf("%s   ·   fuel %d / %d", sel.Kind, p.campaign.FuelCost(sel.ID), p.fuelAvailable())
		mlge_text.Draw(screen, line, 13, infoX, lblY+24, dashBodyColor)
		y := lblY + 46
		if sel.Summary != "" {
			y = drawWrapped(screen, sel.Summary, 12, infoX, y, infoW, 16, color.RGBA{150, 170, 195, 255}) + 6
		}
		if sel.Visited {
			if s := scenario.ByID(sel.ScenarioID); s != nil {
				y = drawWrapped(screen, s.Name+": "+s.Description, 12, infoX, y, infoW, 16, color.RGBA{120, 200, 255, 220}) + 6
			}
		}
		if config.Global().DebugShowScenarioID {
			dbg := fmt.Sprintf("id:%s  scenario:%s  map:%s  seed:%d", sel.ID, sel.ScenarioID, sel.MapID, sel.Seed)
			mlge_text.Draw(screen, dbg, 11, infoX, y, color.RGBA{110, 130, 150, 220})
		}
	}

	if p.status != "" {
		mlge_text.Draw(screen, p.status, 13, 20, sh-30, color.RGBA{230, 160, 90, 255})
	}
}

// ---- procedural icon styling ----

func kindColor(kind string) color.RGBA {
	switch kind {
	case campaign.HomeKind:
		return color.RGBA{255, 220, 120, 255}
	case "planet":
		return color.RGBA{90, 170, 255, 255}
	case "gas_giant":
		return color.RGBA{210, 160, 120, 255}
	case "moon":
		return color.RGBA{185, 195, 205, 255}
	case "asteroid_field":
		return color.RGBA{170, 140, 110, 255}
	case "station":
		return color.RGBA{150, 220, 200, 255}
	default:
		return color.RGBA{140, 200, 220, 255}
	}
}

func kindRadius(kind string) float32 {
	switch kind {
	case campaign.HomeKind:
		return 11
	case "gas_giant":
		return 10
	case "planet":
		return 8
	case "moon":
		return 5
	case "asteroid_field":
		return 4
	default:
		return 6
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// drawDashedLine strokes a dashed segment from (x0,y0) to (x1,y1).
func drawDashedLine(dst *ebiten.Image, x0, y0, x1, y1, dash, gap, width float32, clr color.Color) {
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length <= 0 {
		return
	}
	ux, uy := dx/length, dy/length
	for pos := float32(0); pos < length; pos += dash + gap {
		end := pos + dash
		if end > length {
			end = length
		}
		vector.StrokeLine(dst, x0+ux*pos, y0+uy*pos, x0+ux*end, y0+uy*end, width, clr, true)
	}
}

// entityDisplayName returns a colonist's individual name (with stat levels) for
// the crew lists, or "Colonist" as a fallback.
func entityDisplayName(e *ecs.Entity) string {
	if e == nil {
		return "Colonist"
	}
	name := "Colonist"
	if e.HasComponent(rlcomponents.Description) {
		if dc, ok := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent); ok && dc.Name != "" {
			name = dc.Name
		}
	}
	if e.HasComponent(components.StatProgression) {
		prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
		name += fmt.Sprintf("  [S:%d D:%d I:%d]", prog.Str.Level, prog.Dex.Level, prog.Int.Level)
	}
	return name
}
