package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLooseStackLines(t *testing.T) {
	info := &HoveredTileInfo{LooseStacks: []LooseStack{{Name: "Metal Ore", Quantity: 37}, {Name: "Crystal", Quantity: 1}}}
	assert.Equal(t, []string{"Metal Ore x37", "Crystal x1"}, looseStackLines(info))
	assert.Empty(t, looseStackLines(&HoveredTileInfo{}))
}

// The panel only rebuilds when this says the tile changed, so a pile growing under
// a still cursor has to count as a change.
func TestTileInfoEqual_SeesStackQuantity(t *testing.T) {
	a := HoveredTileInfo{LooseStacks: []LooseStack{{Name: "Metal Ore", Quantity: 3}}}
	b := HoveredTileInfo{LooseStacks: []LooseStack{{Name: "Metal Ore", Quantity: 4}}}
	assert.False(t, tileInfoEqual(a, b))
	assert.True(t, tileInfoEqual(a, a))
}
