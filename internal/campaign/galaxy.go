package campaign

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// The pregenerated galaxy (docs/brainstorm/pregenerated_galaxy_plan.md):
// every system exists from day one, made in one pass from the seed and the
// size, drawing nothing that depends on who arrives. Nothing creates a
// system during play; scans, quests and spreading races only learn of ones
// already there.

// Galaxy is what is true of the galaxy whoever arrives in it: its size and
// shape, where expeditions may arrive (landfalls) and the gate. The systems
// themselves stay in Campaign.Locations and their past in Campaign.History,
// because a Location also carries the expedition's state of it (visited,
// identified, the level save, faction presences); splitting those is the
// later "revisit a galaxy" plan's save change.
type Galaxy struct {
	Size   string  `json:"size"`
	Shape  string  `json:"shape"`
	Radius float64 `json:"radius"`
	// CX, CY is the systems' centre of mass, round which the rim is read.
	CX float64 `json:"cx"`
	CY float64 `json:"cy"`
	// Landfalls are the arrival points on the rim, each owning the system
	// nearest it (bound safe).
	Landfalls []Landfall `json:"landfalls"`
	// GateID is the gate's location (its own site in space, nothing to land
	// on); ControlsID the world beside it that holds its controls.
	GateID     string `json:"gate_id"`
	ControlsID string `json:"controls_id"`
	// GateOn: the gate's controls have been switched on, for everyone, for
	// good (gate.go), by GateOnBy (a faction key, or "player") on GateOnDay.
	GateOn    bool             `json:"gate_on,omitempty"`
	GateOnBy  string           `json:"gate_on_by,omitempty"`
	GateOnDay history.StarDate `json:"gate_on_day,omitempty"`
}

// Landfall is one arrival point: a point just outside the rim and the
// system nearest it, the arriving expedition's first jump.
type Landfall struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	System string  `json:"system"`
}

// GateKind marks the gate: the way out of the galaxy, a site in space with
// nothing to land on. It starts disabled; until its controls are built
// (plan step 4) it is inert.
const GateKind = "gate"

// GateLocationID is the gate's fixed location ID.
const GateLocationID = "gate"

// ShapeName is the galaxy's shape as the Star Map names it ("" without a
// galaxy).
func (c *Campaign) ShapeName() string {
	if c == nil || c.Galaxy == nil {
		return ""
	}
	for _, s := range genConfig().Galaxy.Shapes {
		if s.ID == c.Galaxy.Shape {
			return s.NameDraft
		}
	}
	return c.Galaxy.Shape
}

// GateLocation is the gate, or nil.
func (c *Campaign) GateLocation() *Location {
	if c == nil {
		return nil
	}
	if c.Galaxy != nil {
		if l := c.Locations[c.Galaxy.GateID]; l != nil {
			return l
		}
	}
	return nil
}

// galaxySeed derives the galaxy's random stream from the campaign seed.
func galaxySeed(seed int64) int64 { return seed }

// GenerateGalaxy makes every system of a galaxy of size ("" the default)
// from seed: positions sampled from a rolled shape, the rim and its
// landfalls, the gate and its controls world, then archetypes, names and
// seeds, then every site's history bound in one loop, then their quests.
// It returns a campaign holding only the galaxy (no expedition, no clock
// beyond day zero); GenerateCampaignOpts adds the expedition.
func GenerateGalaxy(seed int64, size string) (*Campaign, error) {
	return generateGalaxy(seed, size, "")
}

// generateGalaxy is GenerateGalaxy with the shape forced (tests; "" rolls it).
func generateGalaxy(seed int64, size, shape string) (*Campaign, error) {
	lt, qt, err := loadGenTemplates()
	if err != nil {
		return nil, err
	}
	cfg := genConfig()
	sz := cfg.Galaxy.size(size)
	c := &Campaign{
		Version:   CampaignVersion,
		Seed:      seed,
		Locations: make(map[string]*Location),
		History:   history.New(seed),
	}
	rng := rand.New(rand.NewSource(galaxySeed(seed)))
	lay, err := layoutGalaxy(rng, cfg, sz, shape)
	if err != nil {
		return nil, err
	}
	g := &Galaxy{Size: sz.ID, Radius: sz.Radius, Shape: lay.shape, CX: lay.cx, CY: lay.cy}
	c.Galaxy = g
	pts := lay.pts
	for _, lf := range lay.landfalls {
		g.Landfalls = append(g.Landfalls, Landfall{X: lf.x, Y: lf.y, System: fmt.Sprintf("sys_%d", lf.sys+1)})
	}
	isLandfall := map[int]bool{}
	for _, lf := range lay.landfalls {
		isLandfall[lf.sys] = true
	}
	safeR := safeRadius(cfg)
	// Every system in a safe start is a start archetype, so none is made a
	// world whose archetype forces a threat (a hive).
	for i, p := range pts {
		for _, lf := range lay.landfalls {
			if math.Hypot(p[0]-lf.x, p[1]-lf.y) <= safeR+1e-6 {
				isLandfall[i] = true
			}
		}
	}
	controls := lay.controls
	gc := cfg.Galaxy

	// 4. Archetypes, seeds and names, from the galaxy stream in order.
	var locs []*Location
	arch := make([]locArchetype, len(pts))
	for i, p := range pts {
		a := weightedArchetype(rng, lt)
		if isLandfall[i] {
			a = startArchetype(rng, lt)
		}
		loc := c.placeLocation(rng, a, p[0], p[1])
		locs = append(locs, loc)
		arch[i] = a
	}
	g.ControlsID = locs[controls].ID
	// The controls world is a planet with ground to dig its vault into (a
	// start archetype), and the gate sits beside it: both from a stream of
	// their own, so every other system's draws stay as they were.
	crng := rand.New(rand.NewSource(galaxySeed(seed)*48271 + 16807))
	ca := startArchetype(crng, lt)
	arch[controls] = ca
	cl := locs[controls]
	cl.Kind, cl.Archetype, cl.MapID, cl.Summary = ca.Kind, ca.ID, pick(crng, ca.Maps), ca.Summary
	cl.QuestTag, cl.Tags = firstTag(ca.Tags), append([]string(nil), ca.Tags...)
	gate := c.placeGate(crng, locs[controls], gc, sz.MinSeparation, safeR)
	g.GateID = gate.ID
	// The controls, deep in a guarded vault: a structure fixture, built with
	// the level and never dropped by the lore budget.
	locs[controls].AddFixture(QuestFixture{Kind: GateControlsKind, Structure: GateVaultStructure, StructW: GateVaultW, StructH: GateVaultH})

	// 5. History, once: every site bound in ID order, then placed.
	for i, loc := range locs {
		c.bindHistory(loc, arch[i].SpawnEvent)
		c.addHistoryQuest(loc)
		c.addStationQuests(loc)
	}

	// 6. Quests bound to real sites: one or two posted, and the datapad clues.
	for i, loc := range locs {
		a := arch[i]
		nq := 1 + rng.Intn(2)
		for i := 0; i < nq; i++ {
			if qd := c.makeQuest(rng, qt, a, loc); qd != nil {
				c.QuestDefs = append(c.QuestDefs, *qd)
			}
		}
		c.addDatapadQuests(rng, qt, a, loc)
	}
	return c, nil
}

// galaxyLayout is a galaxy's geometry before anything is made at it: the
// shape, the system positions, their centre of mass, the landfalls and the
// controls world's index.
type galaxyLayout struct {
	shape     string
	pts       [][2]float64
	cx, cy    float64
	landfalls []landfallPick
	controls  int
}

// layoutGalaxy rolls a galaxy's geometry from rng (shape "" rolls it):
// steps 1 to 3 of GenerateGalaxy.
func layoutGalaxy(rng *rand.Rand, cfg GenConfig, sz GalaxySize, shape string) (galaxyLayout, error) {
	gc := cfg.Galaxy
	rolled := pickShape(rng, gc.Shapes)
	if shape == "" {
		shape = rolled
	}
	build := shapeBuilders[shape]
	if build == nil {
		return galaxyLayout{}, fmt.Errorf("galaxy: no density function for shape %q", shape)
	}
	lay := galaxyLayout{shape: shape}
	lay.pts = samplePoints(rng, build(rng), sz.Systems, sz.Radius, sz.MinSeparation)
	for _, p := range lay.pts {
		lay.cx += p[0]
		lay.cy += p[1]
	}
	lay.cx /= float64(len(lay.pts))
	lay.cy /= float64(len(lay.pts))
	want := cfg.FactionsMax + 1 + max(gc.LandfallSpare, 0)
	safeR := safeRadius(cfg)
	minD := math.Max(gc.LandfallMinDistance, 2*safeR+1)
	lay.landfalls = pickLandfalls(rng, lay.pts, lay.cx, lay.cy, gc.RimBins, want, minD, cfg.StartRadius, safeR)
	isLandfall := map[int]bool{}
	var lfs []Landfall
	for _, lf := range lay.landfalls {
		isLandfall[lf.sys] = true
		lfs = append(lfs, Landfall{X: lf.x, Y: lf.y})
	}
	lay.controls = pickControls(rng, lay.pts, lfs, isLandfall, sz.Radius, safeR)
	return lay, nil
}

func safeRadius(cfg GenConfig) float64 {
	if cfg.SafeStartRadius > 0 {
		return cfg.SafeStartRadius
	}
	return cfg.StartRadius
}

func pickShape(rng *rand.Rand, shapes []GalaxyShape) string {
	total := 0
	for _, s := range shapes {
		total += max(s.Weight, 0)
	}
	if total <= 0 {
		return shapes[0].ID
	}
	r := rng.Intn(total)
	for _, s := range shapes {
		if r < max(s.Weight, 0) {
			return s.ID
		}
		r -= max(s.Weight, 0)
	}
	return shapes[len(shapes)-1].ID
}

// samplePoints draws n min-separated points in the disc of radius R, each
// kept with the density's probability. A shape too tight for the
// separation loosens it a little at a time, so generation never stalls.
func samplePoints(rng *rand.Rand, dens densityFunc, n int, R, sep float64) [][2]float64 {
	pts := make([][2]float64, 0, n)
	tries := 0
	for len(pts) < n {
		tries++
		if tries%(n*400) == 0 {
			sep *= 0.9
		}
		a := rng.Float64() * 2 * math.Pi
		r := math.Sqrt(rng.Float64())
		x, y := math.Cos(a)*r, math.Sin(a)*r
		if rng.Float64() >= dens(x, y) {
			continue
		}
		x, y = x*R, y*R
		ok := true
		for _, p := range pts {
			if math.Hypot(p[0]-x, p[1]-y) < sep {
				ok = false
				break
			}
		}
		if ok {
			pts = append(pts, [2]float64{x, y})
		}
	}
	return pts
}

// landfallNeighbours systems within landfallReach make a system a landfall
// candidate: somewhere to go after the first landing.
const (
	landfallNeighbours = 3
	landfallReach      = 30.0
	// landfallClear keeps a landfall point (the Wreck Site's icon) off every system.
	landfallClear = 5.0
)

type landfallPick struct {
	x, y float64
	sys  int
}

// pickLandfalls reads the rim (the outermost system in each angular bin
// round the centre of mass), then takes up to want rim systems in a rolled
// order whose arrival points (start_radius further out) keep minD apart.
// Fewer than want fit: the distance is relaxed toward the safe minimum.
// Each landfall owns the system nearest its point.
func pickLandfalls(rng *rand.Rand, pts [][2]float64, cx, cy float64, bins, want int, minD, out, safeR float64) []landfallPick {
	rim := make([]int, bins)
	best := make([]float64, bins)
	for i := range rim {
		rim[i] = -1
	}
	// A rim system must have neighbours, so no expedition arrives at an
	// outlier with nowhere to go: the outermost of each bin among those with
	// landfallNeighbours others within landfallReach.
	need := landfallNeighbours
	near := make([]int, len(pts))
	for i, p := range pts {
		for j, q := range pts {
			if i != j && math.Hypot(p[0]-q[0], p[1]-q[1]) <= landfallReach {
				near[i]++
			}
		}
	}
	for i, p := range pts {
		if near[i] < need {
			continue
		}
		a := math.Atan2(p[1]-cy, p[0]-cx)
		b := int((a + math.Pi) / (2 * math.Pi) * float64(bins))
		if b >= bins {
			b = bins - 1
		}
		if d := math.Hypot(p[0]-cx, p[1]-cy); rim[b] < 0 || d > best[b] {
			rim[b], best[b] = i, d
		}
	}
	var cands []landfallPick
	for _, i := range rim {
		if i < 0 {
			continue
		}
		p := pts[i]
		dx, dy := p[0]-cx, p[1]-cy
		d := math.Max(math.Hypot(dx, dy), 1e-9)
		x, y := p[0]+dx/d*out, p[1]+dy/d*out
		// The landfall system is whichever is nearest the point.
		own, nd := i, math.Hypot(p[0]-x, p[1]-y)
		for j, q := range pts {
			if dd := math.Hypot(q[0]-x, q[1]-y); dd < nd {
				own, nd = j, dd
			}
		}
		if near[own] < need || nd < landfallClear {
			continue // the point's own system is an outlier, or the point sits on one
		}
		cands = append(cands, landfallPick{x: x, y: y, sys: own})
	}
	rng.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	floor := 2*safeR + 1
	for {
		var got []landfallPick
		used := map[int]bool{}
		for _, c := range cands {
			if len(got) >= want || used[c.sys] {
				continue
			}
			ok := true
			for _, o := range got {
				if math.Hypot(o.x-c.x, o.y-c.y) < minD {
					ok = false
					break
				}
			}
			if ok {
				got = append(got, c)
				used[c.sys] = true
			}
		}
		if len(got) >= want || minD <= floor {
			return got
		}
		minD = math.Max(minD*0.8, floor)
	}
}

// pickControls is the gate's controls world: a system outside every safe
// start, as far from the nearest landfall point as the galaxy allows (the
// plan asked for at least the radius, which a rim ringed with landfalls
// never allows; see the plan's As built), rolled among those within
// controlsSpread of the farthest.
func pickControls(rng *rand.Rand, pts [][2]float64, lfs []Landfall, isLandfall map[int]bool, R, safeR float64) int {
	minDist := func(p [2]float64) float64 {
		d := math.Inf(1)
		for _, l := range lfs {
			d = math.Min(d, math.Hypot(p[0]-l.X, p[1]-l.Y))
		}
		return d
	}
	// Only systems linked to most landfalls by hops of at most
	// controlsLinkReach: the farthest system is otherwise often one alone in
	// a gap (a ring's core across its void), which no ship could reach.
	linked := linkedToLandfalls(pts, lfs, controlsLinkReach)
	dist := make([]float64, len(pts))
	bestI, bestD := 0, -1.0
	for i, p := range pts {
		dist[i] = -1
		if isLandfall[i] || !linked[i] {
			continue
		}
		if d := minDist(p); d > safeR+1 {
			dist[i] = d
			if d > bestD {
				bestI, bestD = i, d
			}
		}
	}
	var far []int
	for i, d := range dist {
		if d >= 0 && d >= bestD*controlsSpread {
			far = append(far, i)
		}
	}
	if len(far) > 0 {
		return far[rng.Intn(len(far))]
	}
	return bestI
}

// controlsLinkReach is the longest hop that links the controls world to
// the landfalls: a rival colony's ship range (its rival view's base_range).
const controlsLinkReach = 40.0

// linkedToLandfalls marks the systems of the cluster (systems joined by
// hops of at most reach) holding the most landfall points' nearest systems.
func linkedToLandfalls(pts [][2]float64, lfs []Landfall, reach float64) []bool {
	comp := make([]int, len(pts))
	for i := range comp {
		comp[i] = -1
	}
	n := 0
	for i := range pts {
		if comp[i] >= 0 {
			continue
		}
		stack := []int{i}
		comp[i] = n
		for len(stack) > 0 {
			a := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for b := range pts {
				if comp[b] < 0 && math.Hypot(pts[a][0]-pts[b][0], pts[a][1]-pts[b][1]) <= reach {
					comp[b] = n
					stack = append(stack, b)
				}
			}
		}
		n++
	}
	count := make([]int, n)
	for _, l := range lfs {
		best, bd := 0, math.Inf(1)
		for i, p := range pts {
			if d := math.Hypot(p[0]-l.X, p[1]-l.Y); d < bd {
				best, bd = i, d
			}
		}
		count[comp[best]]++
	}
	top := 0
	for k := range count {
		if count[k] > count[top] {
			top = k
		}
	}
	out := make([]bool, len(pts))
	for i := range pts {
		out[i] = comp[i] == top
	}
	return out
}

// controlsSpread is how near the farthest-from-every-landfall distance a
// controls world candidate must be.
const controlsSpread = 0.85

// placeLocation adds one galaxy system of archetype a at (x, y): its ID,
// seed, name and map from the galaxy stream. Its history is bound later,
// with every other system's.
func (c *Campaign) placeLocation(rng *rand.Rand, a locArchetype, x, y float64) *Location {
	c.GenSeq++
	id := fmt.Sprintf("sys_%d", c.GenSeq)
	loc := &Location{
		ID:        id,
		Name:      c.uniqueName(rng),
		Kind:      a.Kind,
		Archetype: a.ID,
		MapID:     pick(rng, a.Maps),
		Seed:      c.Seed + int64(c.GenSeq)*131,
		X:         x,
		Y:         y,
		Summary:   a.Summary,
		QuestTag:  firstTag(a.Tags),
		Tags:      append([]string(nil), a.Tags...),
	}
	c.Locations[id] = loc
	return loc
}

// placeGate puts the gate a short hop from the controls world, clear of
// every other system.
func (c *Campaign) placeGate(rng *rand.Rand, controls *Location, gc GalaxyConfig, sep, safeR float64) *Location {
	lo, hi := gc.GateHopMin, math.Max(gc.GateHopMax, gc.GateHopMin)
	var x, y float64
	for i := 0; i < 64; i++ {
		a := rng.Float64() * 2 * math.Pi
		r := lo + rng.Float64()*(hi-lo)
		x, y = controls.X+math.Cos(a)*r, controls.Y+math.Sin(a)*r
		if !c.tooClose(x, y, sep) && !c.nearLandfall(x, y, safeR) {
			break
		}
	}
	loc := &Location{
		ID: GateLocationID, Name: "The Gate", Kind: GateKind, // DRAFT
		X: x, Y: y, Seed: c.Seed + 9973,
		Summary: "A ring of old machinery the size of a moon, adrift and dark. Whatever switches it on is not here.", // DRAFT
	}
	c.Locations[loc.ID] = loc
	return loc
}

// nearLandfall reports whether (x, y) is within r of any landfall point.
func (c *Campaign) nearLandfall(x, y, r float64) bool {
	if c.Galaxy == nil {
		return false
	}
	for _, l := range c.Galaxy.Landfalls {
		if math.Hypot(l.X-x, l.Y-y) <= r+1e-6 {
			return true
		}
	}
	return false
}

// safeStart reports whether loc's history must be safe: within the safe
// radius of any landfall point (every expedition's first jump). A campaign
// built by hand, with no galaxy, measures from the Wreck Site.
func (c *Campaign) safeStart(loc *Location) bool {
	if c.Galaxy != nil && loc.ID == c.Galaxy.ControlsID {
		return true // its vault's guards are danger enough: never a threat world on top
	}
	if c.Galaxy != nil && len(c.Galaxy.Landfalls) > 0 {
		return c.nearLandfall(loc.X, loc.Y, safeRadius(genConfig()))
	}
	return c.firstJump(loc)
}

// Systems is every system of the galaxy with a site, sorted by ID.
func (c *Campaign) Systems() []*Location {
	var out []*Location
	for _, l := range c.Locations {
		if l.HasSite() && l.Kind != EncounterKind && !l.Tutorial {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LandfallSafe is the systems within the safe radius of landfall i.
func (c *Campaign) LandfallSafe(i int) []*Location {
	if c.Galaxy == nil || i < 0 || i >= len(c.Galaxy.Landfalls) {
		return nil
	}
	l := c.Galaxy.Landfalls[i]
	r := safeRadius(genConfig())
	var out []*Location
	for _, s := range c.Systems() {
		if math.Hypot(s.X-l.X, s.Y-l.Y) <= r+1e-6 {
			out = append(out, s)
		}
	}
	return out
}

// BackdropStar is one speck of the Star Map's galaxy haze: a world position
// and a 0..1 brightness from the shape's density there.
type BackdropStar struct {
	X, Y, Bright float64
}

// backdropPerArea is how many backdrop specks per square unit of the disc.
const backdropPerArea = 0.04

// backdropSalt separates the backdrop's sampling stream from the galaxy's.
const backdropSalt = 0x5eed_ba5e

// GalaxyBackdrop is the faint dust that shows the galaxy's shape before its
// systems are seen: specks sampled from the shape's density function (the
// one the systems were drawn from, replayed from the galaxy seed), never
// from the systems themselves. A function of seed, shape and radius only.
func GalaxyBackdrop(seed int64, shape string, radius float64) []BackdropStar {
	build := shapeBuilders[shape]
	if build == nil || radius <= 0 {
		return nil
	}
	// Replay the layout's stream up to the density function.
	rng := rand.New(rand.NewSource(galaxySeed(seed)))
	pickShape(rng, genConfig().Galaxy.Shapes)
	dens := build(rng)
	srng := rand.New(rand.NewSource(galaxySeed(seed) ^ backdropSalt))
	n := int(math.Pi * radius * radius * backdropPerArea)
	out := make([]BackdropStar, 0, n)
	for tries := 0; len(out) < n && tries < n*200; tries++ {
		a := srng.Float64() * 2 * math.Pi
		r := math.Sqrt(srng.Float64())
		x, y := math.Cos(a)*r, math.Sin(a)*r
		d := dens(x, y)
		if srng.Float64() >= d {
			continue
		}
		out = append(out, BackdropStar{X: x * radius, Y: y * radius, Bright: math.Min(d, 1)})
	}
	return out
}

// Backdrop is this campaign's galaxy haze (nil for a hand-built campaign).
func (c *Campaign) Backdrop() []BackdropStar {
	if c.Galaxy == nil {
		return nil
	}
	return GalaxyBackdrop(c.Seed, c.Galaxy.Shape, c.Galaxy.Radius)
}
