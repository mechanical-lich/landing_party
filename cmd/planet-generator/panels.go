package main

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// slotName prints a tile slot by tile name.
type slotName rllayered.Slot

func (s slotName) String() string {
	slot := rllayered.Slot(s)
	if slot.IsEmpty() {
		return "-"
	}
	return world.TileIndexToName[slot.Type]
}

// buildOptionsPanel is the left panel: everything that feeds generation.
func (a *app) buildOptionsPanel() {
	u, o := &a.ui, a.opts
	u.beginPanel("options", rect{0, 0, leftW, screenH})
	changed := false

	genLabel := "Generate (G)"
	if a.dirty {
		genLabel = "Generate (G) - options changed"
	}
	if u.button(genLabel) {
		a.requestGenerate()
	}
	u.toggle("Regenerate automatically on change", &a.autoGen)

	u.header("Location")
	if u.cycle("Map", o.mapNames, &o.mapIdx) {
		o.selectMap(o.mapIdx)
		changed = true
	}
	if u.cycle("Scenario", o.scenNames, &o.scenarioIdx) {
		o.applyScenario()
		changed = true
	}
	x, w := u.panel.x+panelPad, u.contentWidth()
	u.text("Seed", textSize, x, u.y, w, colDim)
	u.y += 15
	const randW = 70
	if u.intField("seed", rect{x, u.y, w - randW - 4, rowH}, &o.seedText) {
		changed = true
	}
	if u.buttonAt(rect{x + w - randW, u.y, randW, rowH}, "Random", false) {
		o.randomizeSeed()
		changed = true
	}
	u.y += rowH + 6

	if u.toggle("Roll size from seed (as in the game)", &o.rollSize) {
		changed = true
	}
	if u.sliderInt("w", "Width", &o.w, 32, 640) {
		o.rollSize, changed = false, true
	}
	if u.sliderInt("h", "Height", &o.h, 32, 640) {
		o.rollSize, changed = false, true
	}
	if u.sliderInt("z", "Z levels", &o.z, 3, 24) {
		o.rollSize, changed = false, true
	}

	u.header("Terrain")
	if u.cycle("Primer", o.primers, &o.primerIdx) {
		changed = true
	}
	if o.primerIdx >= 0 {
		for _, d := range primerParams[o.primers[o.primerIdx]] {
			v := o.param(d)
			if u.slider("param:"+d.key, d.name, &v, d.min, d.max, d.step) {
				o.md.TerrainParams[d.key] = v
				changed = true
			}
		}
	}

	u.header("Biomes")
	if u.cycle("Biome map", biomeMapTypes, &o.biomeTypeIdx) {
		changed = true
	}
	switch biomeMapTypes[o.biomeTypeIdx] {
	case "perlin_temp_humidity":
		scale := o.md.BiomeMap.Scale
		if scale == 0 {
			scale = 200 // BuildBiomeMap's default
		}
		if u.slider("biomeScale", "Noise scale", &scale, 10, 400, 5) {
			o.md.BiomeMap.Scale = scale
			changed = true
		}
		if u.slider("biomeLat", "Latitude weight", &o.md.BiomeMap.LatitudeWeight, 0, 1, 0.05) {
			changed = true
		}
		for _, id := range o.biomes {
			on := o.biomeOn[id]
			if u.toggle(id, &on) {
				o.biomeOn[id] = on
				changed = true
			}
		}
	case "uniform":
		if u.cycle("Biome", o.biomes, &o.singleBiomeIdx) {
			changed = true
		}
	}
	if u.toggle("Biome features (trees etc.)", &o.biomeFeatures) {
		changed = true
	}

	u.header("Features")
	if len(o.features) == 0 {
		u.label("None on this map", colDim)
	}
	for i, f := range o.features {
		if u.toggle(f.label(), &f.enabled) {
			changed = true
		}
		if !f.enabled {
			continue
		}
		name := "Count"
		if !f.touched && f.block.CountMax > f.block.Count {
			name = fmt.Sprintf("Count (rolled %d-%d)", f.block.Count, f.block.CountMax)
		}
		if u.sliderInt("feat:"+strconv.Itoa(i), name, &f.count, 0, f.sliderMax) {
			f.touched = true
			changed = true
		}
	}

	u.header("Scenario rules")
	if o.sc == nil {
		u.label("No scenario: nothing spawns", colDim)
	} else {
		if u.toggle("Run setup scripts", &o.setupScripts) {
			changed = true
		}
		if u.toggle("Spawn hostiles and fauna", &o.spawns) {
			changed = true
		}
		if o.spawns && u.sliderInt("hostileMax", "Spawn cap", &o.sc.HostileMax, 0, 400) {
			changed = true
		}
	}
	if u.cycle("Lighting", lightModes, &o.lightIdx) {
		changed = true
	}
	if lightModes[o.lightIdx] == "fixed" && u.sliderInt("ambient", "Ambient light", &o.ambient, 0, 100) {
		changed = true
	}

	u.endPanel()
	if changed {
		a.dirty = true
	}
}

// buildToolbar is the strip above the world: simulation and tool controls.
func (a *app) buildToolbar() {
	u := &a.ui
	u.beginPanel("toolbar", rect{leftW, 0, screenW - leftW - rightW, toolbarH})
	y, h := 5, 24
	x := leftW + 8
	btn := func(txt string, w int, on bool) bool {
		r := rect{x, y, w, h}
		x += w + 4
		return u.buttonAt(r, txt, on)
	}
	runTxt := "Run (Space)"
	if a.running {
		runTxt = "Pause (Space)"
	}
	if btn(runTxt, 100, a.running) {
		a.running = !a.running
	}
	if btn("Step (.)", 62, false) {
		a.running = false
		a.lab.Step()
	}
	speed := a.lab.Speed()
	for _, s := range []struct {
		name string
		v    int
	}{{"x1", 1}, {"x2", 2}, {"x4", 4}} {
		if btn(s.name, 30, speed == s.v) {
			a.lab.SetSpeed(s.v)
		}
	}
	x += 8
	if btn("Place Colonist (C)", 124, a.tool == toolPlace) {
		a.toggleTool(toolPlace)
	}
	x += 8
	if btn("Fullbright", 74, a.fullbright) {
		a.fullbright = !a.fullbright
		config.Global().DebugDisableLighting = a.fullbright
	}
	if btn("Paths", 50, a.showPaths) {
		a.showPaths = !a.showPaths
	}
}

// buildInfoPanel is the right panel: overview map, results, inspector, and log.
func (a *app) buildInfoPanel() {
	u := &a.ui
	lvl := a.lab.Level()
	c := a.lab.Camera()
	r := rect{screenW - rightW, 0, rightW, screenH}
	u.beginPanel("info", r)

	// Overview map of the camera's z-level; click to move the camera.
	mapR := rect{r.x + panelPad, u.y, overviewSz, overviewSz}
	u.y += overviewSz + 6
	if u.hit(mapR) && u.down {
		wx := (u.mx - mapR.x) * lvl.GetWidth() / overviewSz
		wy := (u.my - mapR.y) * lvl.GetHeight() / overviewSz
		a.centerOn(wx, wy, c.Z)
		a.follow = false
	}
	u.draw(func(dst *ebiten.Image) { a.drawOverview(dst, mapR) })

	u.label(fmt.Sprintf("Tick %d   Day %d   Hour %d", a.lab.Tick(), lvl.Day, lvl.Hour), colText)
	u.label(fmt.Sprintf("Z %d (surface %d)   zoom %dpx", c.Z, lvl.SurfaceZ, c.TileW), colText)

	if a.selected != nil {
		u.header("Selected")
		a.buildInspector()
	}

	colonists := a.lab.Colonists()
	u.header(fmt.Sprintf("Colonists (%d)", len(colonists)))
	if len(colonists) == 0 {
		u.label("Press C, then click a walkable tile", colDim)
	}
	for i, e := range colonists {
		if i >= 8 {
			u.label(fmt.Sprintf("+%d more", len(colonists)-i), colDim)
			break
		}
		if u.button(colonistRow(e)) {
			a.selected = e
			if pc := position(e); pc != nil {
				a.centerOn(pc.GetX(), pc.GetY(), pc.GetZ())
			}
		}
	}

	u.header("Results")
	u.label(fmt.Sprintf("%dx%dx%d  generated in %dms", lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth(), a.genTime.Milliseconds()), colText)
	u.label(fmt.Sprintf("Hostiles/fauna %d   Entities %d", a.stats.hostiles, a.stats.entities), colText)
	if len(a.stats.biomes) > 0 {
		u.label("Biomes", colDim)
		for _, line := range a.stats.biomes {
			u.label("  "+line, colText)
		}
	}
	if len(a.stats.deposits) > 0 {
		u.label("Deposits (tiles)", colDim)
		for _, id := range countsSorted(a.stats.deposits) {
			u.label(fmt.Sprintf("  %s %d", id, a.stats.deposits[id]), colText)
		}
	}
	u.label(fmt.Sprintf("Mountains %.0f%%   Caverns %d tiles", a.stats.mountainPct, a.stats.cavern), colText)
	if len(a.stats.blueprints) > 0 {
		u.label("Entities by type", colDim)
		for i, id := range countsSorted(a.stats.blueprints) {
			if i >= 14 {
				u.label(fmt.Sprintf("  +%d more types", len(a.stats.blueprints)-i), colDim)
				break
			}
			u.label(fmt.Sprintf("  %s %d", id, a.stats.blueprints[id]), colText)
		}
	}

	u.header("Log")
	log := message.MessageLog
	if len(log) > 8 {
		log = log[len(log)-8:]
	}
	for _, line := range log {
		u.label(line, colDim)
	}
	u.endPanel()
}

func (a *app) drawOverview(dst *ebiten.Image, r rect) {
	c := a.lab.Camera()
	lvl := a.lab.Level()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(r.x), float64(r.y))
	dst.DrawImage(a.overview.GetImage(c.Z), op)

	sx := float32(overviewSz) / float32(lvl.GetWidth())
	sy := float32(overviewSz) / float32(lvl.GetHeight())
	// Entities with minds: colonists green, the rest red.
	for _, e := range lvl.Entities {
		if e == nil || e.HasComponent(rlcomponents.Dead) {
			continue
		}
		pc := position(e)
		if pc == nil || pc.GetZ() != c.Z {
			continue
		}
		var col color.RGBA
		switch {
		case e.HasComponent(components.Worker):
			col = color.RGBA{80, 240, 120, 255}
		case e.HasComponent(components.FactionAI) || e.HasComponent(components.ScriptedAI):
			col = color.RGBA{240, 80, 70, 255}
		default:
			continue
		}
		vector.DrawFilledRect(dst, float32(r.x)+float32(pc.GetX())*sx-1, float32(r.y)+float32(pc.GetY())*sy-1, 3, 3, col, false)
	}
	// Camera rectangle, clipped to the map since the camera can overhang the edges.
	x0 := max(float32(c.X)*sx, 0)
	y0 := max(float32(c.Y)*sy, 0)
	x1 := min(float32(c.X+c.ViewW())*sx, overviewSz)
	y1 := min(float32(c.Y+c.ViewH())*sy, overviewSz)
	if x1 > x0 && y1 > y0 {
		vector.StrokeRect(dst, float32(r.x)+x0, float32(r.y)+y0, x1-x0, y1-y0, 1, color.RGBA{255, 255, 255, 180}, false)
	}
}

// buildInspector shows the state of the selected entity, focused on what its AI
// is doing.
func (a *app) buildInspector() {
	u := &a.ui
	e := a.selected
	name := e.Blueprint
	if e.HasComponent(rlcomponents.Description) {
		if dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent); dc.Name != "" {
			name = dc.Name + " (" + e.Blueprint + ")"
		}
	}
	u.label(name, colText)
	if pc := position(e); pc != nil {
		u.label(fmt.Sprintf("At %d,%d,%d", pc.GetX(), pc.GetY(), pc.GetZ()), colText)
	}
	if e.HasComponent(rlcomponents.Health) {
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		u.label(fmt.Sprintf("HP %d/%d", hc.Health, hc.MaxHealth), colText)
	}
	if e.HasComponent(rlcomponents.AIMemory) {
		mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		state := mem.State
		if state == "" {
			state = "-"
		}
		u.label(fmt.Sprintf("State %s  target %d,%d,%d  path %d", state, mem.TargetX, mem.TargetY, mem.TargetZ, len(mem.CurrentSteps)), colText)
	}
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		u.label("Task: "+colonyTaskName(wc), colText)
	}
	if e.HasComponent(components.Needs) {
		nc := e.GetComponent(components.Needs).(*components.NeedsComponent)
		u.label(fmt.Sprintf("Hunger %d/%d  Exhaustion %d/%d", nc.Hunger, nc.MaxHunger, nc.Exhaustion, nc.MaxExhaustion), colText)
	}
	if e.HasComponent(components.FactionAI) {
		fa := e.GetComponent(components.FactionAI).(*components.FactionAIComponent)
		u.label(fmt.Sprintf("Faction %s (%s)  target:%v", fa.Faction, fa.BehaviorKey, fa.HasTarget), colText)
	}
	if e.HasComponent(components.ScriptedAI) {
		u.label("Script: "+e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent).Script, colText)
	}
	followTxt := "Follow (F)"
	if a.follow {
		followTxt = "Stop following (F)"
	}
	if u.button(followTxt) {
		a.follow = !a.follow
	}
	if u.button("Remove") {
		a.lab.RemoveEntity(e)
		a.selected, a.follow = nil, false
	}
}

// colonistRow is a colonist's one-line summary in the Colonists list.
func colonistRow(e *ecs.Entity) string {
	txt := "colonist"
	if e.HasComponent(rlcomponents.Description) {
		if dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent); dc.Name != "" {
			txt = dc.Name
		}
	}
	if e.HasComponent(rlcomponents.AIMemory) {
		if st := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).State; st != "" {
			txt += " - " + st
		}
	}
	return txt
}

// worldStats is the numbers shown under Results. Terrain figures are computed once
// per generation; entity figures refresh while the simulation runs.
type worldStats struct {
	biomes      []string
	deposits    map[string]int
	mountainPct float64
	cavern      int

	entities   int
	hostiles   int
	blueprints map[string]int
}

func (st *worldStats) compute(lvl *world.Level) {
	w, h, d := lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth()
	st.biomes, st.deposits, st.cavern = nil, map[string]int{}, 0

	tally := map[string]int{}
	for _, b := range lvl.BiomeMap {
		if b != "" {
			tally[b]++
		}
	}
	ids := make([]string, 0, len(tally))
	for id := range tally {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return tally[ids[i]] > tally[ids[j]] })
	for _, id := range ids {
		st.biomes = append(st.biomes, fmt.Sprintf("%s %.0f%%", id, 100*float64(tally[id])/float64(w*h)))
	}

	mountains := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if lvl.SurfaceZ < d && lvl.GetTerrainKind(x, y, lvl.SurfaceZ) == world.TKUnderground {
				mountains++
			}
			for z := 0; z < d; z++ {
				if lvl.GetTerrainKind(x, y, z) == world.TKCavern {
					st.cavern++
				}
				t := lvl.GetTilePtr(x, y, z)
				if t == nil {
					continue
				}
				if name := slotName(t.Middle).String(); world.IsDepositTileName(name) {
					st.deposits[name]++
				}
			}
		}
	}
	st.mountainPct = 100 * float64(mountains) / float64(w*h)
	st.refreshEntities(lvl)
}

func (st *worldStats) refreshEntities(lvl *world.Level) {
	st.entities, st.hostiles = 0, 0
	st.blueprints = map[string]int{}
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			st.entities++
			st.blueprints[e.Blueprint]++
			if e.HasComponent(components.FactionAI) || e.HasComponent(components.ScriptedAI) {
				st.hostiles++
			}
		}
	}
}
