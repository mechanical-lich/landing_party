package view

import "testing"

// baseCamera is a 16px-tile, 640x360 canvas view scrolled to (10,20) on z=3.
func baseCamera() Camera {
	return Camera{X: 10, Y: 20, Z: 3, TileW: 16, TileH: 16, SpriteW: 16, SpriteH: 16, CanvasW: 640, CanvasH: 360}
}

func TestViewSizeDerivedFromCanvas(t *testing.T) {
	c := baseCamera()
	if c.ViewW() != 40 || c.ViewH() != 22 {
		t.Fatalf("view = %dx%d, want 40x22", c.ViewW(), c.ViewH())
	}
	var zero Camera
	if zero.ViewW() != 0 || zero.ViewH() != 0 {
		t.Fatalf("zero-tile camera should have 0 view, got %dx%d", zero.ViewW(), zero.ViewH())
	}
}

func TestWorldToScreenMatchesOldFormula(t *testing.T) {
	c := baseCamera()
	// Old: sx = (wx-CameraX)*TileW, sy = (wy-CameraY)*TileH.
	sx, sy := c.WorldToScreen(12, 25)
	if sx != (12-10)*16 || sy != (25-20)*16 {
		t.Fatalf("WorldToScreen(12,25) = (%d,%d), want (32,80)", sx, sy)
	}
}

func TestWorldToScreenFloatMatchesOldFormula(t *testing.T) {
	c := baseCamera()
	// Old effect math: (wx - cameraX) * tileW on fractional world coords.
	fx, fy := c.WorldToScreenFloat(12.5, 24.25)
	if fx != (12.5-10)*16 || fy != (24.25-20)*16 {
		t.Fatalf("WorldToScreenFloat(12.5,24.25) = (%v,%v), want (40,68)", fx, fy)
	}
}

func TestScreenToWorldMatchesOldFormula(t *testing.T) {
	c := baseCamera()
	// Old: wx = sx/TileW + CameraX, wy = sy/TileH + CameraY.
	wx, wy := c.ScreenToWorld(32, 80)
	if wx != 32/16+10 || wy != 80/16+20 {
		t.Fatalf("ScreenToWorld(32,80) = (%d,%d), want (12,25)", wx, wy)
	}
}

func TestContains(t *testing.T) {
	c := baseCamera() // view covers x:[10,50) y:[20,42) z=3
	cases := []struct {
		x, y, z int
		want    bool
	}{
		{10, 20, 3, true},  // top-left corner
		{49, 41, 3, true},  // bottom-right inside
		{50, 20, 3, false}, // x just past right edge
		{10, 42, 3, false}, // y just past bottom edge
		{9, 20, 3, false},  // x left of view
		{12, 25, 2, false}, // wrong z
	}
	for _, tc := range cases {
		if got := c.Contains(tc.x, tc.y, tc.z); got != tc.want {
			t.Errorf("Contains(%d,%d,%d) = %v, want %v", tc.x, tc.y, tc.z, got, tc.want)
		}
	}
}

func TestCenterOnMatchesOldFormula(t *testing.T) {
	c := baseCamera()
	c.CenterOn(100, 200, 5)

	st := SidebarWidth/16 + 1 // = 14
	wantX := 100 - st - (40-st)/2
	wantY := 200 - 22/2
	if c.X != wantX || c.Y != wantY || c.Z != 5 {
		t.Fatalf("CenterOn = (%d,%d,%d), want (%d,%d,5)", c.X, c.Y, c.Z, wantX, wantY)
	}
}

func TestViewportSnapshot(t *testing.T) {
	c := baseCamera()
	vp := c.Viewport()
	if vp != (Viewport{X: 10, Y: 20, Z: 3, W: 40, H: 22}) {
		t.Fatalf("Viewport = %+v", vp)
	}
}

// TestZoomClampsTileSize guards the div-by-zero risk: repeated zoom-out never
// drives the tile size to 0, and repeated zoom-in never exceeds the max.
func TestZoomClampsTileSize(t *testing.T) {
	c := baseCamera()
	c.TileW, c.TileH = 24, 24 // the game's base tile size

	for i := 0; i < 10; i++ {
		c.ZoomAt(320, 176, false) // keep zooming out well past the bound
	}
	if c.TileW < MinTileSize || c.TileH < MinTileSize {
		t.Fatalf("zoom-out floor breached: TileW=%d TileH=%d, min=%d", c.TileW, c.TileH, MinTileSize)
	}

	for i := 0; i < 10; i++ {
		c.ZoomAt(320, 176, true) // keep zooming in well past the bound
	}
	if c.TileW > MaxTileSize || c.TileH > MaxTileSize {
		t.Fatalf("zoom-in ceiling breached: TileW=%d TileH=%d, max=%d", c.TileW, c.TileH, MaxTileSize)
	}
}

// TestZoomKeepsTileUnderCursor is the property that matters: after zooming, the
// world tile beneath the cursor pixel is unchanged.
func TestZoomKeepsTileUnderCursor(t *testing.T) {
	for _, in := range []bool{true, false} {
		c := baseCamera()
		px, py := 320, 176 // some interior pixel
		beforeX, beforeY := c.ScreenToWorld(px, py)

		c.ZoomAt(px, py, in)

		afterX, afterY := c.ScreenToWorld(px, py)
		if afterX != beforeX || afterY != beforeY {
			t.Errorf("zoom(in=%v): tile under cursor moved from (%d,%d) to (%d,%d)",
				in, beforeX, beforeY, afterX, afterY)
		}
		wantTile := 32
		if in && c.TileW != wantTile {
			t.Errorf("zoom in: TileW = %d, want %d", c.TileW, wantTile)
		}
		// 16 halves to 8, which the [MinTileSize=12] clamp floors to 12.
		if !in && c.TileW != MinTileSize {
			t.Errorf("zoom out: TileW = %d, want %d (clamped)", c.TileW, MinTileSize)
		}
	}
}
