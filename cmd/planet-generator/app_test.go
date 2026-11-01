package main

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/config"
)

// testApp generates a small location, as main does on the first frame.
func testApp(t *testing.T) *app {
	t.Helper()
	cfg := config.Global()
	cfg.ScreenWidth, cfg.ScreenHeight = screenW, screenH
	cfg.WorldWidth, cfg.WorldHeight = screenW, screenH
	o := testOptions(t, "earth_like")
	o.seedText = "4242"
	o.rollSize = false
	o.w, o.h, o.z = 80, 80, 10
	a := &app{opts: o, autoGen: true}
	a.generate()
	if a.lab == nil {
		t.Fatalf("generation failed: %s", a.genErr)
	}
	t.Cleanup(a.lab.Close)
	return a
}

// click runs one frame's widget and world input with the mouse pressed at (x,y).
func (a *app) click(x, y int) {
	a.ui.cmds = a.ui.cmds[:0]
	a.ui.mx, a.ui.my, a.ui.pressed, a.ui.down, a.ui.released = x, y, true, true, false
	a.buildOptionsPanel()
	a.buildToolbar()
	a.buildInfoPanel()
	a.handleWorldInput()
	a.ui.pressed, a.ui.down = false, false
}

// walkableScreenPos finds a free tile in view, returning its tile and screen pixel.
func (a *app) walkableScreenPos(t *testing.T) (tx, ty, sx, sy int) {
	t.Helper()
	c := a.lab.Camera()
	for sy := toolbarH + 12; sy < screenH-statusH; sy += c.TileH {
		for sx := leftW + 12; sx < screenW-rightW; sx += c.TileW {
			wx, wy := c.ScreenToWorld(sx, sy)
			if e, err := a.lab.PlaceColonist(wx, wy, c.Z); err == nil {
				a.lab.RemoveEntity(e)
				return wx, wy, sx, sy
			}
		}
	}
	t.Fatal("no walkable tile in view")
	return
}

func TestToolbarRunAndFullbrightButtons(t *testing.T) {
	a := testApp(t)
	a.click(leftW+20, 15) // Run
	if !a.running {
		t.Error("Run button did not start the simulation")
	}
	a.click(leftW+20, 15)
	if a.running {
		t.Error("Pause button did not stop the simulation")
	}
	before := a.lab.Tick()
	a.click(leftW+8+104+10, 15) // Step
	if a.lab.Tick() != before+1 {
		t.Errorf("Step advanced %d ticks", a.lab.Tick()-before)
	}
	a.click(leftW+8+104+66+34+34+10, 15) // x4
	if a.lab.Speed() != 4 {
		t.Errorf("speed = %d after x4", a.lab.Speed())
	}
}

func TestPlaceColonistToolAndSelection(t *testing.T) {
	a := testApp(t)
	tx, ty, sx, sy := a.walkableScreenPos(t)

	// Without the tool a click on empty ground places nothing.
	a.click(sx, sy)
	if len(a.lab.Colonists()) != 0 {
		t.Fatal("placed a colonist with no tool active")
	}

	a.toggleTool(toolPlace)
	a.click(sx, sy)
	got := a.lab.Colonists()
	if len(got) != 1 {
		t.Fatalf("colonists = %d after placing", len(got))
	}
	if pc := position(got[0]); pc.GetX() != tx || pc.GetY() != ty {
		t.Errorf("placed at %d,%d, clicked %d,%d", pc.GetX(), pc.GetY(), tx, ty)
	}
	if a.selected != got[0] {
		t.Error("placed colonist is not selected")
	}

	// An occupied tile is refused with a note, and clicks on panels never reach the world.
	a.note = ""
	a.click(sx, sy)
	if len(a.lab.Colonists()) != 1 || a.note == "" {
		t.Errorf("second place on the tile: colonists=%d note=%q", len(a.lab.Colonists()), a.note)
	}
	a.tool = toolSelect
	a.selected = nil
	a.click(sx, sy)
	if a.selected != got[0] {
		t.Error("clicking the colonist did not select it")
	}
	a.selected = nil
	a.click(screenW-5, sy)
	if a.selected != nil {
		t.Error("a click on the info panel reached the world")
	}
}

func TestOptionChangeRegenerates(t *testing.T) {
	a := testApp(t)
	old := a.lab
	a.opts.w = 60
	a.dirty = true
	a.requestGenerate()
	if a.dirty || a.genPending == 0 {
		t.Fatalf("dirty=%v pending=%d", a.dirty, a.genPending)
	}
	a.generate()
	if a.lab == old || a.lab.Level().GetWidth() != 60 {
		t.Errorf("not regenerated: width %d", a.lab.Level().GetWidth())
	}
}
