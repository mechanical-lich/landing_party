package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dropMaterial(t *testing.T, level *world.Level, blueprint string, qty, x, y int) {
	t.Helper()
	e, err := factory.Create(blueprint, x, y, 0)
	require.NoError(t, err)
	e.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
	level.AddEntity(e)
}

// revealedFloor is a floor level with fog off, so stacks count as in view.
func revealedFloor() *world.Level {
	level := b0FloorLevel()
	level.RevealAll = true
	return level
}

func stackCam() view.Camera {
	return view.Camera{TileW: 24, TileH: 24, SpriteW: 24, SpriteH: 24, CanvasW: 480, CanvasH: 480}
}

func TestLooseStacksAt_TotalsMergedAndSeparateUnits(t *testing.T) {
	level := revealedFloor()

	// One entity holding many, as a dropped carry leaves it.
	dropMaterial(t, level, "metal_ore", 37, 2, 2)
	assert.Equal(t, []gui.LooseStack{{Name: "Metal Ore", Quantity: 37}}, looseStacksAt(level, 2, 2, 0))

	// Separate single units on one tile, as mining leaves them.
	for i := 0; i < 5; i++ {
		dropMaterial(t, level, "metal_ore", 1, 3, 3)
	}
	assert.Equal(t, []gui.LooseStack{{Name: "Metal Ore", Quantity: 5}}, looseStacksAt(level, 3, 3, 0))

	// Both together, plus a second material listed separately.
	dropMaterial(t, level, "metal_ore", 10, 3, 3)
	dropMaterial(t, level, "crystal", 2, 3, 3)
	assert.ElementsMatch(t,
		[]gui.LooseStack{{Name: "Metal Ore", Quantity: 15}, {Name: "Crystal", Quantity: 2}},
		looseStacksAt(level, 3, 3, 0))

	assert.Empty(t, looseStacksAt(level, 4, 4, 0))
}

// A locker's contents aren't loose: the inspector shows those.
func TestLooseStacksAt_IgnoresContainerContents(t *testing.T) {
	ms := b0Planet(t, "Stack Locker")
	ms.level.RevealAll = true
	b0Stock(t, ms, 2, 2)
	assert.Empty(t, looseStacksAt(ms.level, 2, 2, 0))
	assert.Empty(t, stackLabels(ms.level, stackCam()))
}

func TestStackLabels_DataLevel(t *testing.T) {
	level := revealedFloor()
	dropMaterial(t, level, "metal_ore", 37, 2, 1)
	dropMaterial(t, level, "metal_ore", 1, 3, 1) // a single unit gets no label
	dropMaterial(t, level, "metal_ore", 1, 4, 2) // two separate units total 2
	dropMaterial(t, level, "metal_ore", 1, 4, 2)

	want := []stackLabel{
		{Text: "37", X: 2*24 + 1, Y: 1*24 + 24 - stackLabelSize - 3},
		{Text: "2", X: 4*24 + 1, Y: 2*24 + 24 - stackLabelSize - 3},
	}
	cam := stackCam()
	assert.Equal(t, want, stackLabels(level, cam))

	// The label follows the camera.
	cam.X, cam.Y = 2, 1
	assert.Equal(t, "37", stackLabels(level, cam)[0].Text)
	assert.Equal(t, 1, stackLabels(level, cam)[0].X)

	// Off-view tile, other z-level, and the far-out zoom step all drop labels.
	cam = stackCam()
	cam.X = 3
	assert.Len(t, stackLabels(level, cam), 1)
	cam = stackCam()
	cam.Z = 1
	assert.Empty(t, stackLabels(level, cam))
	cam = stackCam()
	cam.TileW, cam.TileH = view.MinTileSize, view.MinTileSize
	assert.Empty(t, stackLabels(level, cam))
}

func TestStacksHiddenOnTilesNotInView(t *testing.T) {
	level := b0FloorLevel()
	dropMaterial(t, level, "metal_ore", 37, 2, 1)
	level.Visible = make([]bool, level.GetWidth()*level.GetHeight()*level.GetDepth())

	assert.Empty(t, stackLabels(level, stackCam()))
	assert.Empty(t, looseStacksAt(level, 2, 1, 0))
}
