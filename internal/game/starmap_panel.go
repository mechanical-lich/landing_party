package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/audio"
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
	scanBtn     *minui.Button
	beamDownBtn *minui.Button
	beamUpBtn   *minui.Button

	status  string
	pending state.StateInterface

	scanModal  *minui.Modal
	scanLabel  *minui.Label
	scanLabel2 *minui.Label
	scanOkBtn  *minui.Button
	scanOpen   bool

	// Scanner state, refreshed on Enter/scan/travel (not per frame) — it only
	// changes on research or a config edit, both of which route through
	// refreshLocations.
	scanLevel int
	scanCost  int
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

	p.scanBtn = minui.NewButton("sm_scan", "Scan")
	p.scanBtn.SetPosition(880, btnY)
	p.scanBtn.SetSize(180, 30)
	p.scanBtn.OnClick = p.scan

	p.scanModal = minui.NewModal("sm_scan_result", "Scan Report", 480, 170)
	p.scanModal.SetPosition((sw-480)/2, (sh-170)/2)
	p.scanModal.OnClose = func() { p.scanOpen = false }
	p.scanLabel = minui.NewLabel("sm_scan_l1", "")
	p.scanLabel.SetSize(440, 22)
	p.scanLabel.SetPosition(24, 18)
	p.scanModal.AddChild(p.scanLabel)
	p.scanLabel2 = minui.NewLabelWithColor("sm_scan_l2", "", color.RGBA{170, 190, 220, 255})
	p.scanLabel2.SetSize(440, 22)
	p.scanLabel2.SetPosition(24, 52)
	p.scanModal.AddChild(p.scanLabel2)
	p.scanOkBtn = minui.NewButton("sm_scan_ok", "Ok")
	p.scanOkBtn.SetPosition(190, 100)
	p.scanOkBtn.SetSize(100, 28)
	p.scanOkBtn.OnClick = p.closeScanModal
	p.scanModal.AddChild(p.scanOkBtn)
	p.scanModal.SetVisible(false)

	p.refreshLocations()
	p.refreshCrew()
	return p
}

// ---- data ----

func (p *starMapPanel) refreshLocations() {
	// Remember the selected location by ID: DiscoveredLocations comes from a Go
	// map (random order), so we sort for a stable list and restore selection by
	// identity rather than index — otherwise a refresh (e.g. after a scan) would
	// silently jump the selection to a different location.
	prevID := ""
	if i := p.locList.SelectedIndex; i >= 0 && i < len(p.locIDs) {
		prevID = p.locIDs[i]
	}

	locs := p.campaign.DiscoveredLocations()
	sort.Slice(locs, func(a, b int) bool {
		fa, fb := p.campaign.FuelCost(locs[a].ID), p.campaign.FuelCost(locs[b].ID)
		if fa != fb {
			return fa < fb // nearest first (current location, fuel 0, leads)
		}
		return locs[a].ID < locs[b].ID
	})

	p.locIDs = p.locIDs[:0]
	var labels []string
	for _, loc := range locs {
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

	p.locList.SelectedIndex = 0
	for i, id := range p.locIDs {
		if id == prevID {
			p.locList.SelectedIndex = i
			break
		}
	}
	if len(labels) == 0 {
		p.locList.SelectedIndex = -1
	}

	// Cache the scanner state (level + fuel cost) here rather than reading it
	// per frame in Update — it only changes on research or a config edit.
	p.scanLevel = p.campaign.ScannerLevel()
	p.scanCost = p.campaign.ScanFuelCost()
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

// selectByID selects the destination row for the given location ID, if present.
func (p *starMapPanel) selectByID(id string) {
	for i, lid := range p.locIDs {
		if lid == id {
			p.locList.SelectedIndex = i
			return
		}
	}
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

// clusterBoundaryPad is the screen-pixel margin added to the cluster boundary
// so edge sites (and their icons) sit comfortably inside it.
const clusterBoundaryPad = 48

// clusterBounds returns the world-space centre and radius of a circle enclosing
// all discovered non-Home sites (ok=false if there are none). Centred on the
// cluster — not the ship — so it hugs the sites wherever the ship is.
func (p *starMapPanel) clusterBounds() (cx, cy, radius float64, ok bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	n := 0
	for _, id := range p.locIDs {
		loc := p.campaign.Locations[id]
		if loc == nil || loc.Kind == campaign.HomeKind {
			continue
		}
		minX, maxX = math.Min(minX, loc.X), math.Max(maxX, loc.X)
		minY, maxY = math.Min(minY, loc.Y), math.Max(maxY, loc.Y)
		n++
	}
	if n == 0 {
		return 0, 0, 0, false
	}
	cx, cy = (minX+maxX)/2, (minY+maxY)/2
	for _, id := range p.locIDs {
		loc := p.campaign.Locations[id]
		if loc == nil || loc.Kind == campaign.HomeKind {
			continue
		}
		if d := math.Hypot(loc.X-cx, loc.Y-cy); d > radius {
			radius = d
		}
	}
	return cx, cy, radius, true
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

// scan runs one ship-scanner sweep from the current location: it spends fuel,
// then may chart a new nearby world (chance and reach scale with the scanner's
// research level).
func (p *starMapPanel) scan() {
	level := p.campaign.ScannerLevel()
	if level < 1 {
		p.showScanResult("No ship scanner installed.", "Research Ship Scanner to sweep for new worlds.")
		return
	}
	cost := p.campaign.ScanFuelCost()
	if p.fuelAvailable() < cost {
		p.showScanResult("Not enough fuel to scan.", fmt.Sprintf("Have %d, need %d.", p.fuelAvailable(), cost))
		return
	}
	found, ran := p.campaign.Scan(level)
	if !ran {
		// Precondition failed (e.g. templates unavailable) — don't charge fuel.
		p.showScanResult("Scan systems offline.", "Unable to complete the sweep.")
		return
	}
	storage.Deduct(
		storage.ShipProvider{Level: p.wm.ShipLevel()},
		[]string{p.wm.shipSettlementName()},
		map[string]int{"fuel": cost},
	)
	if found != nil {
		audio.PlayGlobal("notify") // a discovery chime, heard regardless of view
		p.showScanResult(
			fmt.Sprintf("Detected: %s", found.Name),
			fmt.Sprintf("A %s, %d fuel away.   (−%d fuel)", found.Kind, p.campaign.FuelCost(found.ID), cost),
		)
	} else {
		p.showScanResult("No new worlds detected.", fmt.Sprintf("(−%d fuel)", cost))
	}
	// Stay on the world we scanned from, not wherever the list re-sorts to.
	p.refreshLocations()
	p.selectByID(p.campaign.CurrentLocationID)
}

// showScanResult opens the scan-report modal with a headline and a detail line.
func (p *starMapPanel) showScanResult(line1, line2 string) {
	p.scanLabel.Text = line1
	p.scanLabel2.Text = line2
	p.scanModal.SetVisible(true)
	p.scanOpen = true
}

func (p *starMapPanel) closeScanModal() {
	p.scanModal.SetVisible(false)
	p.scanOpen = false
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
	// The scan-report modal takes input priority while open.
	if p.scanOpen {
		p.scanModal.Update()
		return
	}
	p.handleCanvasInput()
	p.locList.Update()
	p.locateBtn.Update()
	p.travelBtn.Update()

	// Ship scanner: label shows the fuel cost; disabled without a scanner or
	// enough fuel. Level/cost are cached (see refreshLocations); only fuel is
	// read live here, and it's in-memory.
	if p.scanLevel < 1 {
		p.scanBtn.Text = "Scan (locked)"
		p.scanBtn.SetEnabled(false)
	} else {
		p.scanBtn.Text = fmt.Sprintf("Scan (%d fuel)", p.scanCost)
		p.scanBtn.SetEnabled(p.fuelAvailable() >= p.scanCost)
	}
	p.scanBtn.Update()

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
	// Frame the working cluster, ignoring Home: it's a distant goal (its dashed
	// line shows the direction) and including it drags the fit way out.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	n := 0
	for _, l := range locs {
		if l.Kind == campaign.HomeKind {
			continue
		}
		minX, maxX = math.Min(minX, l.X), math.Max(maxX, l.X)
		minY, maxY = math.Min(minY, l.Y), math.Max(maxY, l.Y)
		n++
	}
	if n == 0 { // only Home discovered — fall back to including it
		for _, l := range locs {
			minX, maxX = math.Min(minX, l.X), math.Max(maxX, l.X)
			minY, maxY = math.Min(minY, l.Y), math.Max(maxY, l.Y)
		}
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
	// Zoom in past a pure fit so the central cluster reads large by default;
	// outer locations are a short pan away.
	p.zoom = clampZoom(p.fitZoom * defaultZoomFactor)
}

// defaultZoomFactor tightens the initial fit view (of the working cluster) so
// the map opens closer on the central sites rather than showing the full extent.
const defaultZoomFactor = 1.6

const (
	minZoom = 0.05
	maxZoom = 64
)

func clampZoom(z float64) float64 {
	if z < minZoom {
		return minZoom
	}
	if z > maxZoom {
		return maxZoom
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
		hitR := float64(siteRadius(loc)) + 8
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

	// Trade routes: a dotted line between each pair of sites the ship has
	// jumped between, darkening the more that route is travelled.
	for _, route := range p.campaign.TravelRoutes() {
		a, b := p.campaign.Locations[route.A], p.campaign.Locations[route.B]
		if a == nil || b == nil {
			continue
		}
		ax, ay := p.worldToScreen(a.X, a.Y)
		bx, by := p.worldToScreen(b.X, b.Y)
		alpha := 45 + (route.Count-1)*30
		if alpha > 220 {
			alpha = 220
		}
		drawDashedLine(canvas, ax, ay, bx, by, 2, 5, 1, color.RGBA{140, 175, 210, uint8(alpha)})
	}

	// Green boundary hugging all discovered non-Home sites; grows as more
	// systems are charted.
	if bx, by, br, ok := p.clusterBounds(); ok {
		scx, scy := p.worldToScreen(bx, by)
		vector.StrokeCircle(canvas, scx, scy, float32(br)*float32(p.zoom)+clusterBoundaryPad, 2, color.RGBA{120, 220, 160, 220}, true)
	}
	if cur != nil {
		cx, cy := p.worldToScreen(cur.X, cur.Y)
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
		r := siteRadius(loc)
		drawSiteIcon(canvas, loc, sx, sy, r)
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
	p.scanBtn.Draw(screen)
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

	if p.scanOpen {
		p.scanModal.Draw(screen)
	}
}

// ---- procedural site icons ----

const (
	siteSizeScale       = 32.0 // map avg-dimension pixels per icon-radius pixel
	minSiteRadius       = 4
	maxSiteRadius       = 22
	defaultSiteRadius   = 13 // shown until a site has been generated (size unknown)
	minStationRadius    = 26 // stations render a map-footprint silhouette, so bigger
	maxStationRadius    = 40
	stationFootprintDim = 16 // resolution of a station's footprint icon grid
)

// siteRadius scales a site's icon by its generated map size (average of W and
// H; Z is always small so it's ignored). Zero dimensions — an unvisited site
// whose size isn't known yet — fall back to a default radius.
func siteRadius(loc *campaign.Location) float32 {
	if loc.MapW <= 0 || loc.MapH <= 0 {
		return defaultSiteRadius
	}
	r := float64(loc.MapW+loc.MapH) / 2 / siteSizeScale
	if loc.Kind == "station" {
		// Stations draw a whole map-footprint silhouette, so render them larger
		// (and within a higher range) than a planet dot for legibility.
		if r < minStationRadius {
			r = minStationRadius
		}
		if r > maxStationRadius {
			r = maxStationRadius
		}
		return float32(r)
	}
	if r < minSiteRadius {
		r = minSiteRadius
	}
	if r > maxSiteRadius {
		r = maxSiteRadius
	}
	return float32(r)
}

// siteColor is a deterministic per-site colour derived from the location seed.
func siteColor(loc *campaign.Location) color.RGBA {
	rng := rand.New(rand.NewSource(loc.Seed*2654435761 + 1))
	return hsvColor(rng.Float64()*360, 0.5, 0.92)
}

// hsvColor converts HSV (h in degrees, s,v in [0,1]) to an opaque RGBA.
func hsvColor(h, s, v float64) color.RGBA {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

// drawStationFootprint renders a station's captured structure grid as a mini
// map-shaped icon within the 2r×2r box centred on (sx,sy). Returns false (so the
// caller falls back to a placeholder) when no footprint has been captured.
func drawStationFootprint(canvas *ebiten.Image, fp []uint16, sx, sy, r float32) bool {
	if len(fp) != stationFootprintDim {
		return false
	}
	any := false
	for _, row := range fp {
		if row != 0 {
			any = true
			break
		}
	}
	if !any {
		return false
	}
	cell := (r * 2) / float32(stationFootprintDim)
	x0, y0 := sx-r, sy-r
	col := color.RGBA{150, 220, 200, 255}
	for row := 0; row < stationFootprintDim; row++ {
		bits := fp[row]
		for c := 0; c < stationFootprintDim; c++ {
			if bits&(1<<uint(c)) != 0 {
				vector.DrawFilledRect(canvas, x0+float32(c)*cell, y0+float32(row)*cell, cell+0.6, cell+0.6, col, false)
			}
		}
	}
	return true
}

// drawSiteIcon renders a location's procedural marker: a coloured disc for
// planets/moons, a scatter of rocks for asteroid fields, and a "?" square for
// stations. Home keeps its distinct gold disc.
func drawSiteIcon(canvas *ebiten.Image, loc *campaign.Location, sx, sy, r float32) {
	// Home is always its distinct gold marker (it's the goal, never "visited").
	if loc.Kind == campaign.HomeKind {
		vector.DrawFilledCircle(canvas, sx, sy, r, color.RGBA{255, 220, 120, 255}, true)
		return
	}
	// Unvisited sites are unknown: a muted disc with a "?" until you travel there.
	if loc.MapW <= 0 || loc.MapH <= 0 {
		vector.DrawFilledCircle(canvas, sx, sy, r, color.RGBA{55, 65, 85, 255}, true)
		vector.StrokeCircle(canvas, sx, sy, r, 1.5, color.RGBA{130, 150, 180, 255}, true)
		mlge_text.Draw(canvas, "?", float64(r)*1.5, int(sx)-int(r*0.4), int(sy)-int(r*0.9), color.RGBA{200, 215, 235, 255})
		return
	}
	switch loc.Kind {
	case "asteroid_field":
		rng := rand.New(rand.NewSource(loc.Seed*40503 + 7))
		rocks := 5 + int(r/3)
		for i := 0; i < rocks; i++ {
			a := rng.Float64() * 2 * math.Pi
			d := rng.Float64() * float64(r)
			rx := sx + float32(math.Cos(a)*d)
			ry := sy + float32(math.Sin(a)*d)
			rr := r*0.20 + float32(rng.Float64())*r*0.15
			g := uint8(120 + rng.Intn(70))
			vector.DrawFilledCircle(canvas, rx, ry, rr, color.RGBA{g, g - 25, g - 45, 255}, true)
		}
	case "station":
		// Once generated, draw the station's actual map footprint; otherwise a
		// "?" square placeholder.
		if drawStationFootprint(canvas, loc.StationFootprint, sx, sy, r) {
			return
		}
		side := int(r * 2)
		box := minui.Rect{X: int(sx) - int(r), Y: int(sy) - int(r), Width: side, Height: side}
		minui.DrawRect(canvas, box, color.RGBA{150, 220, 200, 255})
		minui.DrawRectStroke(canvas, box, 1, color.RGBA{210, 245, 235, 255})
		fs := float64(r) * 1.6
		mlge_text.Draw(canvas, "?", fs, int(sx)-int(r*0.45), int(sy)-int(r*0.95), color.RGBA{20, 40, 40, 255})
	default:
		vector.DrawFilledCircle(canvas, sx, sy, r, siteColor(loc), true)
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
