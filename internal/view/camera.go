package view

// SidebarWidth is the on-planet HUD sidebar width in pixels. The world view is
// centered in the space to its right, so the camera offsets by it when framing
// a target. Keep in sync with the HUD's own sidebar width (gui.hudSidebarW).
const SidebarWidth = 216

// MinTileSize and MaxTileSize bound the zoomable tile pixel size. ZoomAt clamps
// to this range so the tile size can never reach 0 (which would panic the
// screen↔world division). The range spans the clean halving/doubling steps of
// the 24px base tile: 12 → 24 → 48 → 96.
const (
	MinTileSize = 12
	MaxTileSize = 96
)

// Camera maps world tile coordinates to screen pixels (and back) for the
// on-screen world view. It owns the top-left tile (X,Y), the displayed z-level
// (Z), the zoomable tile pixel size, the fixed sprite pixel size, and the render
// canvas size in pixels. The view size in tiles is derived from the canvas and
// tile size so it stays correct across zoom.
type Camera struct {
	X, Y, Z          int // top-left visible tile (X,Y) and displayed z-level
	TileW, TileH     int // tile size in pixels (mutated by zoom)
	SpriteW, SpriteH int // source sprite size in pixels (fixed)
	CanvasW, CanvasH int // render-surface size in pixels (fixed)
}

// ViewW is the view width in whole tiles. Zero when the tile size is unset.
func (c Camera) ViewW() int {
	if c.TileW == 0 {
		return 0
	}
	return c.CanvasW / c.TileW
}

// ViewH is the view height in whole tiles. Zero when the tile size is unset.
func (c Camera) ViewH() int {
	if c.TileH == 0 {
		return 0
	}
	return c.CanvasH / c.TileH
}

// WorldToScreen converts world tile (wx,wy) to its top-left screen pixel.
func (c Camera) WorldToScreen(wx, wy int) (int, int) {
	return (wx - c.X) * c.TileW, (wy - c.Y) * c.TileH
}

// WorldToScreenF is WorldToScreen with float32 results, for the vector-drawing
// overlays (task markers, selection halos) that work in float pixels.
func (c Camera) WorldToScreenF(wx, wy int) (float32, float32) {
	sx, sy := c.WorldToScreen(wx, wy)
	return float32(sx), float32(sy)
}

// WorldToScreenFloat converts a fractional world position to its screen pixel.
// Effects interpolate between tiles (an arrow mid-flight, a laser endpoint), so
// they need float world coordinates rather than integer tiles.
func (c Camera) WorldToScreenFloat(wx, wy float64) (float64, float64) {
	return (wx - float64(c.X)) * float64(c.TileW), (wy - float64(c.Y)) * float64(c.TileH)
}

// ScreenToWorld converts screen pixel (sx,sy) to the world tile under it.
func (c Camera) ScreenToWorld(sx, sy int) (int, int) {
	return sx/c.TileW + c.X, sy/c.TileH + c.Y
}

// Contains reports whether world tile (wx,wy,wz) is inside the current view.
func (c Camera) Contains(wx, wy, wz int) bool {
	return wz == c.Z &&
		wx >= c.X && wx < c.X+c.ViewW() &&
		wy >= c.Y && wy < c.Y+c.ViewH()
}

// Viewport returns the current view rect for FOV/audio gating.
func (c Camera) Viewport() Viewport {
	return Viewport{X: c.X, Y: c.Y, Z: c.Z, W: c.ViewW(), H: c.ViewH()}
}

// SidebarTiles is SidebarWidth expressed in whole tiles, rounded up so the
// world view never draws under the HUD panel.
func (c Camera) SidebarTiles() int {
	return SidebarWidth/c.TileW + 1
}

// CenterOn frames world tile (wx,wy) in the middle of the space to the right of
// the HUD sidebar, on z-level wz.
func (c *Camera) CenterOn(wx, wy, wz int) {
	st := c.SidebarTiles()
	c.X = wx - st - (c.ViewW()-st)/2
	c.Y = wy - c.ViewH()/2
	c.Z = wz
}

// ZoomAt doubles (in) or halves the tile size while keeping the world tile under
// screen pixel (px,py) fixed, so the point beneath the cursor stays put. The new
// tile size is clamped to [MinTileSize, MaxTileSize]; a zoom past a bound leaves
// the size unchanged and repositions to a no-op.
func (c *Camera) ZoomAt(px, py int, in bool) {
	wx, wy := c.ScreenToWorld(px, py)
	if in {
		c.TileW *= 2
		c.TileH *= 2
	} else {
		c.TileW /= 2
		c.TileH /= 2
	}
	c.TileW = clampTileSize(c.TileW)
	c.TileH = clampTileSize(c.TileH)
	c.X = wx - px/c.TileW
	c.Y = wy - py/c.TileH
}

func clampTileSize(t int) int {
	if t < MinTileSize {
		return MinTileSize
	}
	if t > MaxTileSize {
		return MaxTileSize
	}
	return t
}
