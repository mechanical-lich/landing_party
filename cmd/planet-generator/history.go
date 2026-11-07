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
	"github.com/mechanical-lich/landing_party/internal/world"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

// colOrigin marks history event origins on the map and the overview.
var colOrigin = color.RGBA{230, 90, 230, 255}

// siteLore is the generated history of the location being previewed: a
// standalone campaign seeded from the location seed, with this one site bound
// on the map, the same call charting will make (lore plan, step 2).
type siteLore struct {
	h    *history.History
	site *history.SiteHistory
	err  string
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
	u.label(fmt.Sprintf("Arc %s, chapter %s", s.Arc, chapter), colText)
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
	for _, id := range s.Events {
		e := h.Event(id)
		for _, line := range wrapText(h.Render(o.lib, e, "this world"), u.contentWidth()) {
			u.label(line, colText)
		}
		if e.Origin != nil {
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

// drawOriginsOverview marks every origin on the overview map.
func (a *app) drawOriginsOverview(dst *ebiten.Image, r rect, sx, sy float32) {
	for _, p := range a.opts.lore.origins(a.lab.Level()) {
		x := float32(r.x) + float32(p.X)*sx
		y := float32(r.y) + float32(p.Y)*sy
		if x > float32(r.x+r.w) || y > float32(r.y+r.h) {
			continue
		}
		vector.StrokeRect(dst, x-3, y-3, 7, 7, 1.5, colOrigin, false)
	}
}

// drawOriginsWorld outlines origin tiles at the surface (or the level above it).
func (a *app) drawOriginsWorld(screen *ebiten.Image) {
	c := a.lab.Camera()
	lvl := a.lab.Level()
	for _, p := range a.opts.lore.origins(a.lab.Level()) {
		if p.X >= lvl.GetWidth() || p.Y >= lvl.GetHeight() {
			continue
		}
		if surf := lvl.GetSurfaceZ(p.X, p.Y); c.Z != surf && c.Z != surf+1 {
			continue
		}
		sx, sy := c.WorldToScreenF(p.X, p.Y)
		vector.StrokeRect(screen, sx-float32(c.TileW), sy-float32(c.TileH), float32(c.TileW*3), float32(c.TileH*3), 2, colOrigin, false)
	}
}
