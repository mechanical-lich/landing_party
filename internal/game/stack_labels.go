package game

import (
	"image/color"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

const (
	stackLabelSize = 10
	// stackLabelMinTile is the smallest tile size that still fits a label; at
	// the 12px zoom-out step two digits would overrun the tile.
	stackLabelMinTile = 24
)

var (
	stackLabelColor  = color.RGBA{235, 235, 235, 255}
	stackLabelShadow = color.RGBA{0, 0, 0, 255}
)

// stackLabel is one quantity label to draw on the map, in world-image pixels.
type stackLabel struct {
	Text string
	X, Y int
}

// looseQuantity returns how many units a loose material entity holds, and false
// if the entity isn't one. Mined output doesn't merge on the ground: every unit
// is its own entity, and a dropped carry is one entity holding many.
func looseQuantity(e *ecs.Entity) (int, bool) {
	if e == nil || !e.HasComponent(components.Material) || !e.HasComponent(rlcomponents.Position) {
		return 0, false
	}
	qty := e.GetComponent(components.Material).(*components.MaterialComponent).Quantity
	return max(qty, 1), true
}

// stacksVisible mirrors DrawLevel, which draws entities only on visible tiles, so
// neither the hover panel nor a label reveals a pile the map isn't showing.
func stacksVisible(level *world.Level, x, y, z int) bool {
	return config.Global().DebugDisableLighting || level.RevealAll ||
		len(level.Visible) == 0 || level.GetVisible(x, y, z)
}

// looseStacksAt totals the loose materials on a tile per blueprint, in the order
// they were found.
func looseStacksAt(level *world.Level, x, y, z int) []gui.LooseStack {
	if !stacksVisible(level, x, y, z) {
		return nil
	}
	var ents []*ecs.Entity
	level.GetEntitiesAt(x, y, z, &ents)

	var stacks []gui.LooseStack
	var blueprints []string
	for _, e := range ents {
		qty, ok := looseQuantity(e)
		if !ok {
			continue
		}
		if i := slices.Index(blueprints, e.Blueprint); i >= 0 {
			stacks[i].Quantity += qty
			continue
		}
		name := e.Blueprint
		if e.HasComponent(rlcomponents.Description) {
			if n := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).DisplayName(); n != "" {
				name = n
			}
		}
		blueprints = append(blueprints, e.Blueprint)
		stacks = append(stacks, gui.LooseStack{Name: name, Quantity: qty})
	}
	return stacks
}

// stackLabels returns the labels for the current view, ordered top-left to
// bottom-right: one per tile whose loose materials total more than one unit,
// sitting in the tile's bottom-left corner. None when zoomed too far out.
func stackLabels(level *world.Level, cam view.Camera) []stackLabel {
	if cam.TileW < stackLabelMinTile {
		return nil
	}
	type tileKey struct{ x, y int }
	var totals map[tileKey]int
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			qty, ok := looseQuantity(e)
			if !ok {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
			if !cam.Contains(x, y, z) || !stacksVisible(level, x, y, z) {
				continue
			}
			if totals == nil {
				totals = make(map[tileKey]int)
			}
			totals[tileKey{x, y}] += qty
		}
	}

	var keys []tileKey
	for k, total := range totals {
		if total > 1 {
			keys = append(keys, k)
		}
	}
	slices.SortFunc(keys, func(a, b tileKey) int {
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})

	labels := make([]stackLabel, 0, len(keys))
	for _, k := range keys {
		sx, sy := cam.WorldToScreen(k.x, k.y)
		labels = append(labels, stackLabel{
			Text: strconv.Itoa(totals[k]),
			X:    sx + 1,
			Y:    sy + cam.TileH - stackLabelSize - 3,
		})
	}
	return labels
}

// drawStackLabels draws the quantity labels with a one-pixel shadow, so they read
// over any sprite.
func drawStackLabels(screen *ebiten.Image, level *world.Level, cam view.Camera) {
	for _, l := range stackLabels(level, cam) {
		mlge_text.Draw(screen, l.Text, stackLabelSize, l.X+1, l.Y+1, stackLabelShadow)
		mlge_text.Draw(screen, l.Text, stackLabelSize, l.X, l.Y, stackLabelColor)
	}
}
