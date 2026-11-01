package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

var (
	colPanel   = color.RGBA{20, 24, 32, 255}
	colHeader  = color.RGBA{160, 200, 255, 255}
	colText    = color.RGBA{220, 230, 240, 255}
	colDim     = color.RGBA{135, 145, 160, 255}
	colWarn    = color.RGBA{255, 200, 90, 255}
	colTrack   = color.RGBA{50, 58, 72, 255}
	colFill    = color.RGBA{80, 140, 210, 255}
	colButton  = color.RGBA{46, 56, 74, 255}
	colHover   = color.RGBA{66, 80, 104, 255}
	colActive  = color.RGBA{50, 110, 80, 255}
	colFieldBg = color.RGBA{12, 15, 20, 255}
	colFocus   = color.RGBA{110, 170, 240, 255}
)

const (
	textSize   = 11.0
	headerSize = 13.0
	rowH       = 22
	sliderRowH = 28
	panelPad   = 10
	wheelStep  = 40
)

// rect is an ints-only rectangle for layout and hit tests.
type rect struct{ x, y, w, h int }

func (r rect) contains(mx, my int) bool {
	return mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h
}

// ui is a small immediate-mode toolkit: widgets are declared during Update, where
// they read input and edit values directly, and record draw commands that Draw
// replays. Each panel clips its own commands and scrolls independently.
type ui struct {
	mx, my   int
	pressed  bool
	down     bool
	released bool

	wheelY float64

	drag  string // slider being dragged
	focus string // text field receiving keys

	panel  rect
	id     string
	y      int            // layout cursor, in screen space
	scroll map[string]int // per-panel scroll offset
	cmds   []drawCmd
	frame  int
}

type drawCmd struct {
	clip rect
	draw func(dst *ebiten.Image)
}

func (u *ui) begin() {
	u.mx, u.my = ebiten.CursorPosition()
	u.pressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	u.down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	u.released = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	_, u.wheelY = ebiten.Wheel()
	if u.pressed {
		u.focus = ""
	}
	if !u.down && !u.released {
		u.drag = ""
	}
	u.cmds = u.cmds[:0]
	u.frame++
}

func (u *ui) draw(fn func(dst *ebiten.Image)) {
	u.cmds = append(u.cmds, drawCmd{clip: u.panel, draw: fn})
}

func (u *ui) render(screen *ebiten.Image) {
	for _, c := range u.cmds {
		dst := screen.SubImage(image.Rect(c.clip.x, c.clip.y, c.clip.x+c.clip.w, c.clip.y+c.clip.h)).(*ebiten.Image)
		c.draw(dst)
	}
}

// beginPanel starts a panel with a background; id keys its scroll position. The
// wheel scrolls it while the cursor is over it.
func (u *ui) beginPanel(id string, r rect) {
	u.id, u.panel = id, r
	if u.scroll == nil {
		u.scroll = map[string]int{}
	}
	if r.contains(u.mx, u.my) && u.wheelY != 0 {
		u.scroll[id] -= int(u.wheelY * wheelStep)
		if u.scroll[id] < 0 {
			u.scroll[id] = 0
		}
	}
	u.y = r.y + panelPad - u.scroll[id]
	u.draw(func(dst *ebiten.Image) {
		vector.DrawFilledRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colPanel, false)
	})
}

// endPanel clamps the panel's scroll to its content height, leaving room to see
// the bottom row.
func (u *ui) endPanel() {
	contentH := u.y + u.scroll[u.id] - u.panel.y + panelPad
	maxScroll := contentH - u.panel.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if u.scroll[u.id] > maxScroll {
		u.scroll[u.id] = maxScroll
	}
}

// hit reports whether the mouse is over r, within the panel's visible area.
func (u *ui) hit(r rect) bool {
	return u.panel.contains(u.mx, u.my) && r.contains(u.mx, u.my)
}

func (u *ui) contentWidth() int { return u.panel.w - panelPad*2 }

func (u *ui) text(txt string, size float64, x, y, maxW int, col color.Color) {
	u.draw(func(dst *ebiten.Image) {
		mlge_text.DrawClipped(dst, txt, size, x, y, maxW, col)
	})
}

func (u *ui) gap(px int) { u.y += px }

// header draws a section title.
func (u *ui) header(txt string) {
	u.y += 8
	u.text(txt, headerSize, u.panel.x+panelPad, u.y, u.contentWidth(), colHeader)
	u.y += 20
}

// label draws one line of text.
func (u *ui) label(txt string, col color.Color) {
	u.text(txt, textSize, u.panel.x+panelPad, u.y+2, u.contentWidth(), col)
	u.y += 16
}

// buttonAt declares a button in box r, reporting a click. on shows it latched.
func (u *ui) buttonAt(r rect, txt string, on bool) bool {
	hover := u.hit(r)
	bg := colButton
	switch {
	case on:
		bg = colActive
	case hover:
		bg = colHover
	}
	u.draw(func(dst *ebiten.Image) {
		vector.DrawFilledRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), bg, false)
		tw, _ := mlge_text.Measure(txt, textSize)
		mlge_text.DrawClipped(dst, txt, textSize, r.x+(r.w-int(tw))/2, r.y+r.h/2-6, r.w-4, colText)
	})
	return hover && u.pressed
}

// button declares a full-width button row.
func (u *ui) button(txt string) bool {
	r := rect{u.panel.x + panelPad, u.y, u.contentWidth(), rowH}
	u.y += rowH + 4
	return u.buttonAt(r, txt, false)
}

// toggle declares a checkbox row, reporting a change.
func (u *ui) toggle(txt string, v *bool) bool {
	r := rect{u.panel.x + panelPad, u.y, u.contentWidth(), 18}
	u.y += 20
	changed := false
	if u.hit(r) && u.pressed {
		*v = !*v
		changed = true
	}
	on := *v
	u.draw(func(dst *ebiten.Image) {
		vector.StrokeRect(dst, float32(r.x)+.5, float32(r.y+3)+.5, 12, 12, 1, colDim, false)
		if on {
			vector.DrawFilledRect(dst, float32(r.x+3), float32(r.y+6), 7, 7, colFill, false)
		}
		mlge_text.DrawClipped(dst, txt, textSize, r.x+18, r.y+2, r.w-18, colText)
	})
	return changed
}

// cycle declares a "name  < value >" picker, reporting a change of *idx.
func (u *ui) cycle(name string, items []string, idx *int) bool {
	const arrowW = 22
	x, w := u.panel.x+panelPad, u.contentWidth()
	u.text(name, textSize, x, u.y, w, colDim)
	u.y += 15
	left := rect{x, u.y, arrowW, rowH}
	right := rect{x + w - arrowW, u.y, arrowW, rowH}
	middle := rect{x + arrowW + 2, u.y, w - 2*arrowW - 4, rowH}
	u.y += rowH + 4
	changed := false
	if u.buttonAt(left, "<", false) && len(items) > 0 {
		*idx = (*idx - 1 + len(items)) % len(items)
		changed = true
	}
	if u.buttonAt(right, ">", false) && len(items) > 0 {
		*idx = (*idx + 1) % len(items)
		changed = true
	}
	val := ""
	if *idx >= 0 && *idx < len(items) {
		val = items[*idx]
	}
	u.draw(func(dst *ebiten.Image) {
		vector.DrawFilledRect(dst, float32(middle.x), float32(middle.y), float32(middle.w), float32(middle.h), colFieldBg, false)
		mlge_text.DrawClipped(dst, val, textSize, middle.x+6, middle.y+middle.h/2-6, middle.w-8, colText)
	})
	return changed
}

// slider declares a labelled slider over [min,max], snapped to step, reporting a
// change. id must be unique among sliders on screen.
func (u *ui) slider(id, name string, v *float64, min, max, step float64) bool {
	x, w := u.panel.x+panelPad, u.contentWidth()
	track := rect{x, u.y + 16, w, 8}
	hitBox := rect{x, u.y + 12, w, 16}
	u.y += sliderRowH

	if u.hit(hitBox) && u.pressed {
		u.drag = id
	}
	changed := false
	if u.drag == id && u.down {
		t := float64(u.mx-track.x) / float64(track.w)
		t = math.Max(0, math.Min(1, t))
		nv := min + t*(max-min)
		if step > 0 {
			nv = math.Round(nv/step) * step
		}
		nv = math.Max(min, math.Min(max, nv))
		if nv != *v {
			*v = nv
			changed = true
		}
	}

	val := *v
	valTxt := formatStep(val, step)
	frac := 0.0
	if max > min {
		frac = math.Max(0, math.Min(1, (val-min)/(max-min)))
	}
	u.draw(func(dst *ebiten.Image) {
		mlge_text.DrawClipped(dst, name, textSize, x, track.y-14, w-60, colDim)
		tw, _ := mlge_text.Measure(valTxt, textSize)
		mlge_text.Draw(dst, valTxt, textSize, x+w-int(tw), track.y-14, colText)
		vector.DrawFilledRect(dst, float32(track.x), float32(track.y+2), float32(track.w), 4, colTrack, false)
		vector.DrawFilledRect(dst, float32(track.x), float32(track.y+2), float32(float64(track.w)*frac), 4, colFill, false)
		kx := float32(track.x) + float32(float64(track.w)*frac)
		vector.DrawFilledRect(dst, kx-2, float32(track.y), 4, 8, colText, false)
	})
	return changed
}

// sliderInt is slider for an int value.
func (u *ui) sliderInt(id, name string, v *int, min, max int) bool {
	f := float64(*v)
	if u.slider(id, name, &f, float64(min), float64(max), 1) {
		*v = int(f)
		return true
	}
	return false
}

// intField declares a focusable number field, reporting edits to *txt.
func (u *ui) intField(id string, r rect, txt *string) bool {
	if u.hit(r) && u.pressed {
		u.focus = id
	}
	focused := u.focus == id
	changed := false
	if focused {
		for _, c := range ebiten.AppendInputChars(nil) {
			if (c >= '0' && c <= '9') || (c == '-' && *txt == "") {
				*txt += string(c)
				changed = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(*txt) > 0 {
			*txt = (*txt)[:len(*txt)-1]
			changed = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			u.focus = ""
		}
	}
	shown := *txt
	if focused && u.frame/30%2 == 0 {
		shown += "|"
	}
	u.draw(func(dst *ebiten.Image) {
		vector.DrawFilledRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colFieldBg, false)
		if focused {
			vector.StrokeRect(dst, float32(r.x)+.5, float32(r.y)+.5, float32(r.w)-1, float32(r.h)-1, 1, colFocus, false)
		}
		mlge_text.DrawClipped(dst, shown, textSize, r.x+6, r.y+r.h/2-6, r.w-8, colText)
	})
	return changed
}

// formatStep prints a value with just enough decimals for its slider step.
func formatStep(v, step float64) string {
	if step >= 1 {
		return strconv.Itoa(int(math.Round(v)))
	}
	if step >= 0.1 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}
