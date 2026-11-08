package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// pngScale is the pixels per tile of a headless dump.
const pngScale = 4

// writeLevelPNGs builds the options' location once, as the window would, and
// writes one PNG per level: tiles by kind, entities as dots, a station's
// modules outlined, origins in magenta; plus a text index of the modules.
func writeLevelPNGs(o *options, dir string) error {
	o.rollHistory()
	spec := o.spec()
	lab, err := game.NewPlanetLab(spec)
	if err != nil {
		return err
	}
	defer lab.Close()
	lvl := lab.Level()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var idx strings.Builder
	fmt.Fprintf(&idx, "%s seed %s %dx%dx%d scenario %s\n", spec.Map.ID, o.seedText, lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth(), scenarioName(spec))
	if o.lore.site != nil {
		fmt.Fprintf(&idx, "history: arc %s, tier %s, chapter %s\n", o.lore.site.Arc, o.lore.tier, o.lore.site.Chapter)
	}
	for _, m := range lvl.Modules {
		fmt.Fprintf(&idx, "module %-12s %-10s z%d at %d,%d %dx%d %q event %s damage %d sealed %s\n", m.ID, m.Kind, m.Z, m.X, m.Y, m.W, m.H, m.Name, m.Event, m.Damage, m.Sealed)
	}
	if o.lore.site != nil {
		for _, id := range o.lore.site.Events {
			if x, y, z, ok := generation.Origin(lvl, id); ok {
				mod := ""
				if m := lvl.ModuleAt(x, y, z); m != nil {
					mod = " in " + m.ID
				}
				fmt.Fprintf(&idx, "origin %s (%s) at %d,%d,%d%s\n", id, o.lore.h.Event(id).Type, x, y, z, mod)
			}
		}
	}
	// The bounds of everything built, through every level, for the montage.
	bx0, by0, bx1, by1 := lvl.GetWidth(), lvl.GetHeight(), 0, 0
	for z := 0; z < lvl.GetDepth(); z++ {
		for y := 0; y < lvl.GetHeight(); y++ {
			for x := 0; x < lvl.GetWidth(); x++ {
				if t := lvl.GetTilePtr(x, y, z); t != nil && !t.Floor.IsEmpty() {
					bx0, by0, bx1, by1 = min(bx0, x), min(by0, y), max(bx1, x), max(by1, y)
				}
			}
		}
	}
	if bx1 < bx0 {
		bx0, by0, bx1, by1 = 0, 0, lvl.GetWidth()-1, lvl.GetHeight()-1
	}
	bx0, by0 = max(bx0-2, 0), max(by0-2, 0)
	bx1, by1 = min(bx1+2, lvl.GetWidth()-1), min(by1+2, lvl.GetHeight()-1)
	cw, ch := (bx1-bx0+1)*pngScale, (by1-by0+1)*pngScale
	montage := image.NewRGBA(image.Rect(0, 0, (cw+8)*lvl.GetDepth(), ch))
	for z := 0; z < lvl.GetDepth(); z++ {
		img := image.NewRGBA(image.Rect(0, 0, lvl.GetWidth()*pngScale, lvl.GetHeight()*pngScale))
		for y := 0; y < lvl.GetHeight(); y++ {
			for x := 0; x < lvl.GetWidth(); x++ {
				fill(img, x, y, tileColor(lvl.GetTilePtr(x, y, z)))
			}
		}
		for _, e := range entities(lvl) {
			if e.z == z {
				dot(img, e.x, e.y, e.c)
			}
		}
		for _, m := range lvl.Modules {
			if m.Z == z {
				outline(img, m.X, m.Y, m.W, m.H, color.RGBA{240, 160, 60, 255})
			}
		}
		if o.lore.site != nil {
			for _, id := range o.lore.site.Events {
				if x, y, oz, ok := generation.Origin(lvl, id); ok && oz == z {
					outline(img, x-1, y-1, 3, 3, colOrigin)
				}
			}
		}
		for py := 0; py < ch; py++ {
			for px := 0; px < cw; px++ {
				montage.Set(z*(cw+8)+px, py, img.At(bx0*pngScale+px, by0*pngScale+py))
			}
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("z%d.png", z)))
		if err != nil {
			return err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	f, err := os.Create(filepath.Join(dir, "decks.png"))
	if err != nil {
		return err
	}
	if err := png.Encode(f, montage); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.WriteFile(filepath.Join(dir, "index.txt"), []byte(idx.String()), 0o644)
}

func scenarioName(spec game.PlanetLabSpec) string {
	if spec.Scenario == nil {
		return "none"
	}
	return spec.Scenario.ID
}

type ecsEntity struct {
	x, y, z int
	c       color.RGBA
}

func entities(lvl *world.Level) []*ecsEntity {
	var out []*ecsEntity
	for _, e := range append(append(lvl.Entities[:0:0], lvl.Entities...), lvl.StaticEntities...) {
		if e == nil || !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		c := color.RGBA{200, 200, 255, 255}
		switch {
		case e.HasComponent(rlcomponents.Door):
			c = color.RGBA{60, 220, 220, 255}
		case strings.Contains(e.Blueprint, "light"):
			c = color.RGBA{255, 255, 120, 255}
		case strings.Contains(e.Blueprint, "generator"):
			c = color.RGBA{255, 60, 60, 255}
		case e.Blueprint == "datapad":
			c = color.RGBA{255, 120, 255, 255}
		case e.HasComponent(rlcomponents.Health) && !e.HasComponent(rlcomponents.Solid):
			c = color.RGBA{255, 40, 40, 255}
		}
		out = append(out, &ecsEntity{pc.GetX(), pc.GetY(), pc.GetZ(), c})
	}
	return out
}

func tileColor(t *world.Tile) color.RGBA {
	if t == nil {
		return color.RGBA{0, 0, 0, 255}
	}
	mid, flo := "", ""
	if !t.Middle.IsEmpty() {
		mid = world.TileDefinitions[t.Middle.Type].Name
	}
	if !t.Floor.IsEmpty() {
		flo = world.TileDefinitions[t.Floor.Type].Name
	}
	switch {
	case mid == "stairs_up":
		return color.RGBA{80, 255, 80, 255}
	case mid == "stairs_down":
		return color.RGBA{40, 160, 40, 255}
	case mid == "space" || (mid == "" && flo == ""):
		return color.RGBA{8, 8, 20, 255}
	case mid == "hull_wall":
		return color.RGBA{150, 150, 165, 255}
	case mid == "rubble_pile":
		return color.RGBA{120, 90, 60, 255}
	case mid == "radioactive_ore":
		return color.RGBA{120, 255, 40, 255}
	case strings.Contains(mid, "wires") || strings.Contains(flo, "wires"):
		return color.RGBA{60, 90, 255, 255}
	case mid != "" && t.IsSolid():
		return color.RGBA{110, 110, 110, 255}
	case flo == "dirt_floor":
		return color.RGBA{110, 80, 40, 255}
	case flo == "bunker_floor":
		return color.RGBA{70, 70, 80, 255}
	}
	return color.RGBA{45, 48, 60, 255}
}

func fill(img *image.RGBA, x, y int, c color.RGBA) {
	for dy := 0; dy < pngScale; dy++ {
		for dx := 0; dx < pngScale; dx++ {
			img.SetRGBA(x*pngScale+dx, y*pngScale+dy, c)
		}
	}
}

func dot(img *image.RGBA, x, y int, c color.RGBA) {
	for dy := 1; dy < pngScale-1; dy++ {
		for dx := 1; dx < pngScale-1; dx++ {
			img.SetRGBA(x*pngScale+dx, y*pngScale+dy, c)
		}
	}
}

func outline(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	x0, y0, x1, y1 := x*pngScale, y*pngScale, (x+w)*pngScale-1, (y+h)*pngScale-1
	for px := x0; px <= x1; px++ {
		img.SetRGBA(px, y0, c)
		img.SetRGBA(px, y1, c)
	}
	for py := y0; py <= y1; py++ {
		img.SetRGBA(x0, py, c)
		img.SetRGBA(x1, py, c)
	}
}
