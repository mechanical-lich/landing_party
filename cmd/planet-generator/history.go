package main

import (
	"errors"
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/world"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

// colOrigin marks history event origins on the map and the overview;
// colLanding the landing reserve, colKeepOut its keep-outs, colStamp the
// footprints of the stamps the build placed.
var (
	colStamp   = color.RGBA{240, 160, 60, 200}
	colOrigin  = color.RGBA{230, 90, 230, 255}
	colLanding = color.RGBA{90, 200, 255, 255}
	colKeepOut = color.RGBA{90, 200, 255, 110}
)

// siteLore is the generated history of the location being previewed: a
// standalone campaign seeded from the location seed, with this one site bound
// on the map, the same call charting will make (lore plan, step 2).
type siteLore struct {
	h    *history.History
	site *history.SiteHistory
	err  string
	// ev is the site's evidence as the game will build it, within the lore
	// budget; tier its chapter tier (lore plan, step 8d).
	ev   history.SiteEvidence
	tier string
}

// rollHistory binds a history for the current map and seed. With fromHistory on
// it also switches the scenario to the history's chapter, as the game will.
func (o *options) rollHistory() {
	o.lore = siteLore{}
	if o.lib == nil {
		o.lore.err = "history data not loaded"
		return
	}
	id := o.mapDefs[o.mapIdx].ID
	h := history.New(o.seed())
	site, err := h.BindSite(o.lib, id, o.seed())
	if errors.Is(err, history.ErrNoHistory) {
		o.lore.err = "this map has no history"
		return
	}
	if err != nil {
		o.lore.err = err.Error()
		return
	}
	o.lore.h, o.lore.site = h, site
	o.lore.ev = h.Evidence(o.lib, site)
	o.lore.tier = h.TierOf(o.lib, site)
	if o.fromHistory && (o.sc == nil || o.sc.ID != site.Scenario) {
		o.selectScenarioByID(site.Scenario)
	}
}

// origins are the origin tiles of the site's events, oldest first: where the
// level resolved them when it was built from the history, else as rolled.
func (l siteLore) origins(lvl *world.Level) []history.SitePoint {
	var out []history.SitePoint
	if l.site == nil {
		return nil
	}
	for _, id := range l.site.Events {
		if e := l.h.Event(id); e != nil && e.Origin != nil {
			p := *e.Origin
			if lvl != nil {
				if x, y, _, ok := generation.Origin(lvl, id); ok {
					p = history.SitePoint{X: x, Y: y}
				}
			}
			out = append(out, p)
		}
	}
	return out
}

// originLabel describes what stands at an event's origin: its type, each kept
// evidence block there (a stamp's script and key params) and what the budget
// dropped. short keeps the type and scripts only, for the map.
func (l siteLore) originLabel(id string, short bool, lvl *world.Level) string {
	e := l.h.Event(id)
	if e == nil {
		return id
	}
	parts := []string{e.Type}
	for _, fb := range l.ev.Features {
		if fb.At != "origin:"+id || fb.Kind == "scatter_entity" {
			continue // the relic beside a structure goes unsaid
		}
		if fb.Kind != "stamp" {
			if !short {
				parts = append(parts, fb.Kind)
			}
			continue
		}
		what, _ := fb.Params["script"].(string)
		if !placedAt(lvl, id, fb) {
			what += " (not placed)"
		}
		if !short {
			for _, k := range []string{"class", "purpose", "tech", "damage", "occupants"} {
				if v, ok := fb.Params[k]; ok && v != "" && v != "none" {
					what += fmt.Sprintf(" %s=%v", k, v)
				}
			}
		}
		parts = append(parts, what)
	}
	for _, d := range l.ev.Dropped {
		if d.Event == id && !short {
			parts = append(parts, strings.TrimSpace(fmt.Sprintf("dropped %s %s (%s)", d.Kind, d.Script, d.Limit)))
		}
	}
	return strings.Join(parts, "; ")
}

// placedAt reports whether a level built from the history placed an origin's
// stamp (a footprint of its size anchored at the resolved origin). On a level
// not built from it there is nothing to check, and it reports true.
func placedAt(lvl *world.Level, id string, fb mapdef.FeatureBlock) bool {
	if lvl == nil {
		return true
	}
	x, y, _, ok := generation.Origin(lvl, id)
	if !ok {
		return true
	}
	w, _ := fb.Params["w"].(float64)
	h, _ := fb.Params["h"].(float64)
	for _, f := range lvl.StampFootprints {
		if f[4] == x && f[5] == y && f[2] == int(w) && f[3] == int(h) {
			return true
		}
	}
	return false
}

// budgetLines are the site's lore budget readout: what its kept evidence
// spends against its size class's limits.
func (l siteLore) budgetLines(lvl *world.Level) []string {
	b := l.ev.Budget
	pct := 0.0
	if b.Width*b.Height > 0 {
		pct = 100 * float64(b.Footprint) / float64(b.Width*b.Height)
	}
	lines := []string{
		fmt.Sprintf("Budget %s (rolled %dx%d)", b.Size, b.Width, b.Height),
		fmt.Sprintf("  stamps %d/%d  footprint %.1f%%/%.0f%%", b.Stamps, b.Limits.Stamps, pct, b.Limits.FootprintPercent),
		fmt.Sprintf("  hostiles %d/%d  underground %d/%d  radiation %d/%d",
			b.Hostiles, b.Limits.Hostiles, b.Underground, b.Limits.Underground, b.Radiation, b.Limits.Radiation),
	}
	if lvl != nil && (lvl.GetWidth() != b.Width || lvl.GetHeight() != b.Height) {
		lines = append(lines, fmt.Sprintf("  (this preview is %dx%d: Roll size to match)", lvl.GetWidth(), lvl.GetHeight()))
	}
	if n := len(l.ev.Dropped); n > 0 {
		lines = append(lines, fmt.Sprintf("  %d evidence block(s) dropped", n))
	}
	return lines
}

// keepOuts are the distances (tiles beyond the landing reserve box) origins
// keep: the budget's inert and hostile keep-outs and any event's own.
func (l siteLore) keepOuts() []int {
	b := generation.Budget()
	seen := map[int]bool{b.KeepOut: true, b.HazardKeepOut: true}
	out := []int{b.KeepOut, b.HazardKeepOut}
	for _, k := range l.ev.KeepOut {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// resolved reports whether the level was built from the history (its origins
// were resolved on it).
func (l siteLore) resolved(lvl *world.Level) bool {
	if l.site == nil || lvl == nil {
		return false
	}
	for _, id := range l.site.Events {
		if _, _, _, ok := generation.Origin(lvl, id); ok {
			return true
		}
	}
	return false
}

// buildHistory lists the site's chronicle in the info panel. Events with an
// origin get a button that frames it.
func (a *app) buildHistory() {
	u, o := &a.ui, a.opts
	u.header("History")
	if o.lore.site == nil {
		u.label(o.lore.err, colDim)
		return
	}
	s, h := o.lore.site, o.lore.h
	chapter := "quiet"
	if s.Chapter != "" {
		chapter = h.Event(s.Chapter).Type
	}
	u.label(fmt.Sprintf("Arc %s, chapter %s, tier %s", s.Arc, chapter, o.lore.tier), colText)
	scen := "Plays " + s.Scenario
	if a.opts.sc == nil || a.opts.sc.ID != s.Scenario {
		scen += " (not the one generated)"
	}
	u.label(scen, colText)
	st := s.State
	u.label(fmt.Sprintf("Life %s, %s, %s", st.Life, st.Population, st.Status), colDim)
	if len(st.Remains)+len(st.Contaminants) > 0 {
		u.label(strings.Join(append(append([]string{}, st.Remains...), st.Contaminants...), ", "), colDim)
	}
	lvl := a.lab.Level()
	for _, line := range o.lore.budgetLines(lvl) {
		u.label(line, colDim)
	}
	if lx, ly, lz, ok := generation.Landing(lvl); ok {
		b := generation.Budget()
		if u.button(fmt.Sprintf("Go to landing %d,%d (box %d, keep-outs %v)", lx, ly, 2*b.LandingHalf+1, o.lore.keepOuts())) {
			a.centerOn(lx, ly, lz)
			a.follow = false
		}
	}
	u.gap(3)
	for _, id := range s.Events {
		e := h.Event(id)
		for _, line := range wrapText(h.Render(o.lib, e, "this world"), u.contentWidth()) {
			u.label(line, colText)
		}
		if e.Origin != nil {
			for _, line := range wrapText("at origin: "+o.lore.originLabel(id, false, lvl), u.contentWidth()-8) {
				u.label("  "+line, colOrigin)
			}
			p := e.Origin
			if x, y, _, ok := generation.Origin(lvl, id); ok {
				p = &history.SitePoint{X: x, Y: y}
			}
			if p.X >= lvl.GetWidth() || p.Y >= lvl.GetHeight() {
				u.label(fmt.Sprintf("  origin %d,%d is off this map", p.X, p.Y), colWarn)
			} else if u.button(fmt.Sprintf("Go to origin %d,%d", p.X, p.Y)) {
				z := lvl.GetSurfaceZ(p.X, p.Y) + 1
				if z < 0 || z >= lvl.GetDepth() {
					z = a.lab.Camera().Z
				}
				a.centerOn(p.X, p.Y, z)
				a.follow = false
			}
		}
		u.gap(3)
	}
	if o.lore.resolved(lvl) {
		u.label("Built from this history: evidence and datapads", colDim)
		u.label("at the resolved origins (magenta).", colDim)
	} else {
		u.label("Origins as rolled (magenta). Turn on Scenario", colDim)
		u.label("from history to build the site's evidence.", colDim)
	}
}

// wrapText splits txt into lines that fit maxW pixels at the panel text size.
func wrapText(txt string, maxW int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(txt) {
		next := w
		if cur != "" {
			next = cur + " " + w
		}
		if wd, _ := mlge_text.Measure(next, textSize); int(wd) > maxW && cur != "" {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// drawOriginsOverview marks every origin on the overview map, and the
// landing reserve with its keep-outs.
func (a *app) drawOriginsOverview(dst *ebiten.Image, r rect, sx, sy float32) {
	if lx, ly, _, ok := generation.Landing(a.lab.Level()); ok {
		half := generation.Budget().LandingHalf
		box := func(k int, col color.RGBA, w float32) {
			n := half + k
			x0 := max(float32(r.x), float32(r.x)+float32(lx-n)*sx)
			y0 := max(float32(r.y), float32(r.y)+float32(ly-n)*sy)
			x1 := min(float32(r.x+r.w), float32(r.x)+float32(lx+n+1)*sx)
			y1 := min(float32(r.y+r.h), float32(r.y)+float32(ly+n+1)*sy)
			vector.StrokeRect(dst, x0, y0, x1-x0, y1-y0, w, col, false)
		}
		for _, k := range a.opts.lore.keepOuts() {
			box(k, colKeepOut, 1)
		}
		box(0, colLanding, 1.5)
	}
	for _, f := range a.lab.Level().StampFootprints {
		vector.StrokeRect(dst, float32(r.x)+float32(f[0])*sx, float32(r.y)+float32(f[1])*sy,
			float32(f[2])*sx, float32(f[3])*sy, 1, colStamp, false)
	}
	for _, p := range a.opts.lore.origins(a.lab.Level()) {
		x := float32(r.x) + float32(p.X)*sx
		y := float32(r.y) + float32(p.Y)*sy
		if x > float32(r.x+r.w) || y > float32(r.y+r.h) {
			continue
		}
		vector.StrokeRect(dst, x-3, y-3, 7, 7, 1.5, colOrigin, false)
	}
}

// drawOriginsWorld outlines origin tiles at the surface (or the level above
// it), labelled with the event and what stands there, and the landing reserve
// with its keep-outs.
func (a *app) drawOriginsWorld(screen *ebiten.Image) {
	c := a.lab.Camera()
	lvl := a.lab.Level()
	if lx, ly, lz, ok := generation.Landing(lvl); ok && (c.Z == lz || c.Z == lz+1) {
		half := generation.Budget().LandingHalf
		box := func(k int, col color.RGBA, w float32) {
			n := half + k
			x0, y0 := c.WorldToScreenF(lx-n, ly-n)
			x1, y1 := c.WorldToScreenF(lx+n+1, ly+n+1)
			vector.StrokeRect(screen, x0, y0, x1-x0, y1-y0, w, col, false)
		}
		for _, k := range a.opts.lore.keepOuts() {
			box(k, colKeepOut, 1)
		}
		box(0, colLanding, 2)
		x0, y0 := c.WorldToScreenF(lx-half, ly-half)
		mapLabel(screen, "landing reserve", x0, y0-14, colLanding)
	}
	for _, f := range lvl.StampFootprints {
		x0, y0 := c.WorldToScreenF(f[0], f[1])
		x1, y1 := c.WorldToScreenF(f[0]+f[2], f[1]+f[3])
		vector.StrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, colStamp, false)
	}
	if a.opts.lore.site == nil {
		return
	}
	ids := a.opts.lore.originIDs()
	for i, p := range a.opts.lore.origins(a.lab.Level()) {
		if p.X >= lvl.GetWidth() || p.Y >= lvl.GetHeight() {
			continue
		}
		if surf := lvl.GetSurfaceZ(p.X, p.Y); c.Z != surf && c.Z != surf+1 {
			continue
		}
		sx, sy := c.WorldToScreenF(p.X, p.Y)
		vector.StrokeRect(screen, sx-float32(c.TileW), sy-float32(c.TileH), float32(c.TileW*3), float32(c.TileH*3), 2, colOrigin, false)
		if i < len(ids) {
			id := ids[i]
			mapLabel(screen, a.opts.lore.originLabel(id, true, lvl), sx-float32(c.TileW), sy-float32(c.TileH)-14, colOrigin)
		}
	}
}

// originIDs are the event IDs of origins(), in the same order.
func (l siteLore) originIDs() []string {
	var out []string
	if l.site == nil {
		return nil
	}
	for _, id := range l.site.Events {
		if e := l.h.Event(id); e != nil && e.Origin != nil {
			out = append(out, id)
		}
	}
	return out
}

// mapLabel draws a small label on a dark backing at (x, y).
func mapLabel(dst *ebiten.Image, txt string, x, y float32, col color.Color) {
	const size = 10
	w, h := mlge_text.Measure(txt, size)
	vector.DrawFilledRect(dst, x-2, y-1, float32(w)+4, float32(h)+2, color.RGBA{0, 0, 0, 170}, false)
	mlge_text.Draw(dst, txt, size, int(x), int(y), col)
}
