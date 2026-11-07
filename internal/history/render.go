package history

import (
	"fmt"
	"hash/fnv"
	"log"
	"regexp"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// FormatDate renders a star date for the chronicle: prehistory counts back from
// the campaign's start ("landfall"), play counts forward.
func FormatDate(d StarDate) string {
	switch {
	case d >= 0:
		return fmt.Sprintf("Star date %d", int(d))
	case d > -365:
		return fmt.Sprintf("%d days before landfall", int(-d))
	case d > -730:
		return "A year before landfall"
	default:
		return fmt.Sprintf("%d years before landfall", int(-d)/365)
	}
}

// Render fills an event's text template from its civs, figures and vars.
// siteName names the planet ("" renders as "this world").
func (h *History) Render(lib *Library, ev *Event, siteName string) string {
	return h.render(lib, ev, siteName, false)
}

// RenderSide is Render for a datapad: a per-side event's text_side (its
// leading people's own account) when it has one.
func (h *History) RenderSide(lib *Library, ev *Event, siteName string) string {
	return h.render(lib, ev, siteName, true)
}

func (h *History) render(lib *Library, ev *Event, siteName string, side bool) string {
	def := lib.Events[ev.Type]
	if def == nil {
		return fmt.Sprintf("%s: %s", FormatDate(ev.Date), ev.Type)
	}
	text := def.Text
	switch {
	case side && def.TextSide != "":
		text = def.TextSide
	case ev.Vars[meanwhileVar] != "" && def.TextMeanwhile != "":
		text = def.TextMeanwhile
	}
	return h.fill(lib, ev, siteName, text)
}

// fill renders a template with an event's fills (date, site, civs, war,
// figures, vars).
func (h *History) fill(lib *Library, ev *Event, siteName, text string) string {
	if siteName == "" {
		siteName = ev.Vars["place"]
	}
	if siteName == "" {
		siteName = "this world"
	}
	fills := []string{"{date}", FormatDate(ev.Date), "{site}", siteName}
	civName := func(i int) string {
		if i < len(ev.Civs) {
			if c := h.Civs[ev.Civs[i]]; c != nil {
				return c.Name
			}
		}
		return "unknown"
	}
	fills = append(fills, "{civ}", civName(0), "{enemy}", civName(1))
	during := ""
	if ev.Type != WarEventType && len(ev.Civs) > 0 {
		if w := h.WarAt(ev.Civs[0], ev.Date); w != nil {
			during = fmt.Sprintf(", during the %s–%s war", h.civName0(w, 0), h.civName0(w, 1))
		}
	}
	fills = append(fills, "{during_war}", during)
	warEnd := ""
	if ev.Type == WarEventType {
		if end := WarEnd(ev); end == 0 {
			warEnd = "; it is still being fought"
		} else {
			warEnd = "; it ended " + strings.ToLower(FormatDate(end)[:1]) + FormatDate(end)[1:]
		}
	}
	fills = append(fills, "{war_end}", warEnd)
	for _, id := range ev.Figures {
		if f := h.Figures[id]; f != nil {
			fills = append(fills, "{"+f.Role+"}", f.Name)
		}
	}
	for k, v := range ev.Vars {
		if k == meanwhileVar {
			continue
		}
		if k == "tech" {
			if td := lib.Tech(v); td != nil && td.Label != "" {
				v = td.Label
			}
		}
		fills = append(fills, "{"+k+"}", v)
	}
	return fixArticles(strings.NewReplacer(fills...).Replace(text))
}

var articleBeforeVowel = regexp.MustCompile(`\b([Aa]) ([AEIOU])`)

// fixArticles turns "a Ixithvex" into "an Ixithvex" once names are filled in.
func fixArticles(s string) string {
	return articleBeforeVowel.ReplaceAllString(s, "${1}n $2")
}

func (h *History) civName0(ev *Event, i int) string {
	if i < len(ev.Civs) {
		if c := h.Civs[ev.Civs[i]]; c != nil {
			return c.Name
		}
	}
	return "unknown"
}

// Summary is a site's one line for the Star Map: its arc's summary ({civ} the
// site's civ, {tech} the label of the highest level it reached), then its
// chapter's omen. "" if the arc has none.
func (h *History) Summary(lib *Library, site *SiteHistory) string {
	if site == nil {
		return ""
	}
	arc := lib.Arcs[site.Arc]
	if arc == nil || arc.Summary == "" {
		return ""
	}
	civ, tech := "unknown", "unknown age"
	if c := h.Civs[site.Civ]; c != nil {
		civ = c.Name
		if td := lib.Tech(c.Tech); td != nil && td.Label != "" {
			tech = td.Label
		}
	}
	out := fixArticles(strings.NewReplacer("{civ}", civ, "{tech}", tech).Replace(arc.Summary))
	if site.Chapter != "" {
		if ev := h.Event(site.Chapter); ev != nil {
			if def := lib.Events[ev.Type]; def != nil && def.Omen != "" {
				out += " " + def.Omen
			}
		}
	}
	return out
}

// SiteEvidence is what a site's history adds to generation: the evidence
// features of its events (anchored ones keyed to their event's origin), the
// rolled origins by event ID and each origin's keep-out from the landing, the
// chapter event's ID, the events that leave a lore datapad (every event with
// an origin), and what the site's lore budget dropped.
type SiteEvidence struct {
	Features []mapdef.FeatureBlock
	Origins  map[string][2]int
	KeepOut  map[string]int
	Chapter  string
	Datapads []string
	Dropped  []DroppedEvidence
	// Budget is what the kept evidence spent against the site's size class
	// (the planet generator's readout, lore plan step 8d).
	Budget EvidenceBudget
}

// EvidenceBudget is a site's lore budget and what its kept evidence spent.
type EvidenceBudget struct {
	Width, Height                                       int                    // the map's rolled size
	Size                                                string                 // its size class: small, medium, large
	Limits                                              generation.BudgetClass // the class's limits
	MaxFootprint                                        int                    // FootprintPercent of the surface, in tiles
	Stamps, Footprint, Underground, Hostiles, Radiation int
}

// DroppedEvidence is an evidence block the lore budget left out.
type DroppedEvidence struct {
	Event  string // event ID
	Type   string // event type
	Kind   string // feature kind
	Script string // a stamp's script
	Limit  string // the limit it broke
}

// evidenceCost is what one evidence block spends of a site's lore budget.
type evidenceCost struct {
	stamps, footprint, underground, hostiles, radiation int
}

// spawnerKinds are the feature kinds that place creatures.
var spawnerKinds = map[string]bool{"cave_spawner": true, "buried_spawner": true, "fauna_spawner": true, "derelict_pod": true}

// maxCount is a block's largest possible count.
func maxCount(fb mapdef.FeatureBlock) int {
	n := fb.Count
	if n <= 0 {
		n = 1
	}
	if fb.CountMax > n {
		n = fb.CountMax
	}
	return n
}

// guardHostiles is what a stamp's default "guards" occupants spawn at most: a
// brute and four grunts.
const guardHostiles = 5

func costOf(fb mapdef.FeatureBlock) evidenceCost {
	var c evidenceCost
	n := maxCount(fb)
	switch {
	case fb.Kind == "stamp":
		c.stamps = n
		if k, ok := fb.Params["stamps"].(float64); ok && k > 1 {
			c.stamps = n * int(k) // a mothership's field counts three
		}
		c.footprint = n * paramInt(fb.Params["w"], 9) * paramInt(fb.Params["h"], 7)
		if script, _ := fb.Params["script"].(string); strings.HasPrefix(script, "underground_") {
			c.underground = n
		}
		if occ, _ := fb.Params["occupants"].(string); occ == "" || occ == "guards" {
			c.hostiles = n * guardHostiles
		}
	case spawnerKinds[fb.Kind]:
		c.hostiles = n
	}
	// A block may declare what it brings (spore pods, munitions, turrets).
	if k, ok := fb.Params["hostiles"].(float64); ok && k > 0 {
		c.hostiles = int(k)
	}
	switch {
	case fb.Kind == "radiation_pocket":
		c.radiation = n
	}
	return c
}

// paramInt reads a JSON (float64) or Go (int) number param.
func paramInt(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return def
}

// spawnsHostiles reports whether an event's evidence places creatures.
func spawnsHostiles(def *EventDef) bool {
	for _, e := range def.Evidence {
		if costOf(e.FeatureBlock).hostiles > 0 {
			return true
		}
	}
	return false
}

// Evidence collects a site's generation inputs, in event order, within the
// site's lore budget (generation.Budget, by its map's rolled width). The
// chapter's evidence is spent first: its structures always stay, its
// countable blocks (radiation pockets, creatures) clamp to the limits. The
// rest is kept newest first, and a block that would break a limit is dropped
// (and logged). A dropped block keeps its event's origin and datapad.
func (h *History) Evidence(lib *Library, site *SiteHistory) SiteEvidence {
	var out SiteEvidence
	if site == nil {
		return out
	}
	out.Chapter = site.Chapter
	b := generation.Budget()
	width, height := 0, 0
	if md := lib.Maps[site.Map]; md != nil {
		width, height, _ = md.Size.Roll(site.Seed)
	}
	class := b.For(width)
	sizeName := "large"
	for i, c := range b.Classes {
		if c == class {
			sizeName = []string{"small", "medium", "large"}[min(i, 2)]
		}
	}
	type block struct {
		ev  *Event
		def *EventDef
		fb  mapdef.FeatureBlock
	}
	var blocks []block
	for _, id := range site.Events {
		ev := h.Event(id)
		if ev == nil {
			continue
		}
		def := lib.Events[ev.Type]
		if def == nil {
			continue
		}
		if ev.Origin != nil {
			if out.Origins == nil {
				out.Origins = map[string][2]int{}
				out.KeepOut = map[string]int{}
			}
			out.Origins[ev.ID] = [2]int{ev.Origin.X, ev.Origin.Y}
			out.Datapads = append(out.Datapads, ev.ID)
			k := def.KeepOut
			if k <= 0 {
				k = b.KeepOut
				if def.ChapterTier() != "" && ev.ID == site.Chapter || spawnsHostiles(def) {
					k = b.HazardKeepOut
				}
			}
			out.KeepOut[ev.ID] = k
		}
		for _, e := range def.Evidence {
			if e.Kind == "lore_datapad" {
				continue // every event with an origin leaves one (Datapads)
			}
			fb := e.FeatureBlock
			if fb.At == "origin" {
				if ev.Origin == nil {
					continue
				}
				fb.At = "origin:" + ev.ID
			}
			if fb.Kind == "stamp" {
				var ok bool
				if fb, ok = h.stampFor(lib, site, ev, fb, sizeName); !ok {
					role, _ := e.Params["script"].(string)
					out.Dropped = append(out.Dropped, DroppedEvidence{Event: ev.ID, Type: ev.Type, Kind: fb.Kind, Script: role, Limit: "tech"})
					continue // the datapad stays
				}
			}
			blocks = append(blocks, block{ev, def, fb})
		}
	}
	// Spend the budget: the chapter first, then the newest events.
	keep := make([]bool, len(blocks))
	var spent evidenceCost
	maxFootprint := int(class.FootprintPercent / 100 * float64(width*height))
	// clamp fits a chapter's countable block (radiation pockets, spawned
	// creatures) to what its category has left; its structures always stay.
	clamp := func(i int) {
		fb := &blocks[i].fb
		c := costOf(*fb)
		left := -1
		switch {
		case c.radiation > 0:
			left = class.Radiation - spent.radiation
		case c.hostiles > 0 && c.stamps == 0:
			left = class.Hostiles - spent.hostiles
		}
		if left < 0 || maxCount(*fb) <= left {
			return
		}
		if left == 0 {
			return // spend drops it
		}
		was := maxCount(*fb)
		if fb.Count > left {
			fb.Count = left
		}
		if fb.CountMax > left {
			fb.CountMax = left
		}
		log.Printf("lore budget: %s (%s, %d wide) clamped the %s of chapter %s %s from %d to %d",
			site.ID, site.Map, width, fb.Kind, blocks[i].ev.Type, blocks[i].ev.ID, was, left)
	}
	spend := func(i int, force bool) {
		if force {
			clamp(i)
		}
		c := costOf(blocks[i].fb)
		limit := ""
		switch {
		case spent.stamps+c.stamps > class.Stamps && c.stamps > 0:
			limit = "stamps"
		case width > 0 && spent.footprint+c.footprint > maxFootprint && c.footprint > 0:
			limit = "footprint"
		case spent.underground+c.underground > class.Underground && c.underground > 0:
			limit = "underground"
		case spent.hostiles+c.hostiles > class.Hostiles && c.hostiles > 0:
			limit = "hostiles"
		case spent.radiation+c.radiation > class.Radiation && c.radiation > 0:
			limit = "radiation"
		}
		// A chapter's structure always stays; anything else over a limit
		// (a chapter's block clamped to nothing included) is dropped.
		if limit != "" && !(force && c.stamps > 0) {
			script, _ := blocks[i].fb.Params["script"].(string)
			d := DroppedEvidence{Event: blocks[i].ev.ID, Type: blocks[i].ev.Type, Kind: blocks[i].fb.Kind, Script: script, Limit: limit}
			out.Dropped = append(out.Dropped, d)
			what := d.Kind
			if script != "" {
				what += " " + script
			}
			log.Printf("lore budget: %s (%s, %d wide) dropped the %s of %s %s: over the %s limit",
				site.ID, site.Map, width, what, d.Type, d.Event, limit)
			return
		}
		keep[i] = true
		spent.stamps += c.stamps
		spent.footprint += c.footprint
		spent.underground += c.underground
		spent.hostiles += c.hostiles
		spent.radiation += c.radiation
	}
	for i := range blocks {
		if blocks[i].ev.ID == site.Chapter {
			spend(i, true)
		}
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].ev.ID != site.Chapter {
			spend(i, false)
		}
	}
	out.Budget = EvidenceBudget{Width: width, Height: height, Size: sizeName, Limits: class, MaxFootprint: maxFootprint,
		Stamps: spent.stamps, Footprint: spent.footprint, Underground: spent.underground,
		Hostiles: spent.hostiles, Radiation: spent.radiation}
	relicked := map[string]bool{}
	for i, b := range blocks {
		if !keep[i] {
			continue
		}
		out.Features = append(out.Features, b.fb)
		// A people's structure keeps one of their relics at its origin.
		if b.fb.Kind == "stamp" && strings.HasPrefix(b.fb.At, "origin:") && !relicked[b.ev.ID] {
			if r := relicFor(lib, b.ev); r != "" {
				relicked[b.ev.ID] = true
				out.Features = append(out.Features, mapdef.FeatureBlock{Kind: "scatter_entity", Count: 1, At: b.fb.At,
					Params: map[string]any{"blueprint": r, "kind": "surface"}})
			}
		}
	}
	return out
}

// roleDefaultPurpose is a role's purpose when the event names none.
var roleDefaultPurpose = map[string]string{
	"dwelling": "dwelling", "seat": "seat", "fort": "fort", "shrine": "temple",
	"works": "mine_office", "grave": "grave", "battlefield": "battlefield", "landing": "landing",
}

// ruinous events wreck what stands at their origin: a structure an earlier
// event built there is stamped at damage 2 or worse.
var ruinous = map[string]bool{
	"civ_collapses": true, "nuclear_war": true, "impact_cataclysm": true, "colony_starves": true,
	"reactor_meltdown": true, "mutual_ruin": true, "mutual_annihilation": true, "one_side_prevails": true,
	"ice_age": true, "nanite_conversion": true, "hive_consumption": true, "stellar_flare": true,
}

// stampFor fills a history stamp's params (the structure library's
// contract): an "@role" resolves through the event's tech level (none at
// that level drops the stamp); a crashed_ship takes its class from its
// purpose; the footprint follows the map's size class; the stamp gets the
// event, civ, species, tech, purpose, holds, clue, room names, size and
// damage. ok is false when the stamp is dropped.
func (h *History) stampFor(lib *Library, site *SiteHistory, ev *Event, fb mapdef.FeatureBlock, size string) (mapdef.FeatureBlock, bool) {
	params := make(map[string]any, len(fb.Params)+12)
	for k, v := range fb.Params {
		params[k] = v
	}
	level := ev.Vars["tech"]
	script, _ := params["script"].(string)
	purpose := ev.Vars["purpose"]
	if p, _ := params["purpose"].(string); purpose == "" && p != "" {
		purpose = p
	}
	if strings.HasPrefix(script, "@") {
		role := script[1:]
		td := lib.Tech(level)
		// A role with alternatives picks one per stamp, by the site and the
		// event, so a re-render builds the same.
		var st TechStructure
		if td != nil {
			st = td.Structures[role].Pick(fmt.Sprintf("%d/%s", site.Seed, ev.ID))
		}
		if st.Script == "" {
			return fb, false
		}
		params["script"] = st.Script
		if w, hh := st.Footprint(size); w > 0 {
			params["w"], params["h"] = float64(w), float64(hh)
		}
		if purpose == "" {
			purpose = roleDefaultPurpose[role]
		}
	}
	if params["script"] == "crashed_ship" {
		class, _ := params["class"].(string)
		if pd, ok := lib.Purposes[purpose]; ok && pd.Class != "" {
			class = pd.Class
		}
		if sc, ok := lib.ShipClasses[class]; ok && sc.LargeOnly && size != "large" {
			class = "warship"
		}
		if sc, ok := lib.ShipClasses[class]; ok {
			params["class"] = class
			params["w"], params["h"] = float64(sc.W), float64(sc.H)
			if sc.Stamps > 1 {
				params["stamps"] = float64(sc.Stamps)
			}
		}
	}
	if level != "" {
		params["tech"] = level
	}
	params["event"] = ev.ID
	params["size"] = size
	if len(ev.Civs) > 0 {
		if c := h.Civs[ev.Civs[0]]; c != nil {
			params["civ"] = c.Name
			if c.Species != "" {
				params["species"] = c.Species
			}
		}
	}
	if pd, ok := lib.Purposes[purpose]; ok {
		params["purpose"] = purpose
		if _, set := params["holds"]; !set && pd.Holds != "" {
			params["holds"] = pd.Holds
		}
		for k, v := range pd.Rooms {
			params[k] = v
		}
		if pd.Clue != "" {
			params["clue"] = h.fill(lib, ev, "", pd.Clue)
		}
	}
	params["damage"] = float64(h.stampDamage(site, ev, params))
	fb.Params = params
	return fb, true
}

// stampDamage is how worn a history structure is: its block's own damage,
// at least 1 on a fallen or dead world, at least 2 when a later ruinous
// event (a collapse, a meltdown, a war's end) happened at its origin.
func (h *History) stampDamage(site *SiteHistory, ev *Event, params map[string]any) int {
	d := 0
	if v, ok := params["damage"].(float64); ok {
		d = int(v)
	}
	if site.State.Status == StatusFallen || site.State.Status == StatusDead {
		d = max(d, 1)
	}
	if ev.Origin == nil {
		return d
	}
	later := false
	for _, id := range site.Events {
		if id == ev.ID {
			later = true
			continue
		}
		if o := h.Event(id); later && o != nil && ruinous[o.Type] && o.Origin != nil && *o.Origin == *ev.Origin {
			d = max(d, 2)
		}
	}
	return min(d, 3)
}

// relicFor is the relic an event's people leave at their structure: one of
// its tech level's relics, chosen by the event ID; "" without a level.
func relicFor(lib *Library, ev *Event) string {
	td := lib.Tech(ev.Vars["tech"])
	if td == nil || len(td.Relics) == 0 {
		return ""
	}
	h := fnv.New32a()
	h.Write([]byte(ev.ID))
	return td.Relics[int(h.Sum32())%len(td.Relics)]
}

// LoreFlags are the site's history as setup-script flags: the chapter's event
// type and civ name, and what the history left (has_wreck, has_reactor,
// population). The resolved origin tile is added by the game once the map is
// built (origin_x, origin_y, origin_z). Empty strings and 0 for a site with no
// chapter or civ.
func (h *History) LoreFlags(lib *Library, site *SiteHistory) map[string]any {
	f := map[string]any{
		"origin_event": "",
		"origin_civ":   "",
		"has_wreck":    0.0,
		"has_reactor":  0.0,
		"population":   PopNever,
	}
	if site == nil {
		return f
	}
	f["population"] = site.State.Population
	if Has(site.State.Remains, "wrecks") {
		f["has_wreck"] = 1.0
	}
	if Has(site.State.Remains, "reactor") {
		f["has_reactor"] = 1.0
	}
	civ := site.Civ
	if site.Chapter != "" {
		if ev := h.Event(site.Chapter); ev != nil {
			f["origin_event"] = ev.Type
			if len(ev.Civs) > 0 {
				civ = ev.Civs[0]
			}
			if def := lib.Events[ev.Type]; def != nil {
				for k, v := range def.Flags {
					f[k] = v
				}
			}
		}
	}
	if c := h.Civs[civ]; c != nil {
		f["origin_civ"] = c.Name
		f["tech"] = c.Tech
	}
	f["tier"] = h.TierOf(lib, site)
	return f
}
