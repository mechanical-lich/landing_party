package world

import "math/rand"

// depositRichnessRange returns the [min, max] total-yield range for a mineable
// deposit tile type, and ok=false for anything that isn't a deposit. Ore is the
// richest; crystal and radioactive are scarcer.
func depositRichnessRange(tileName string) (min, max int, ok bool) {
	switch tileName {
	case "ore_deposit":
		return 25, 100, true
	case "crystal_vein":
		return 15, 60, true
	case "radioactive_ore":
		return 10, 40, true
	// Ship rubble (debris piles / damaged walls) is a small scrap-metal deposit:
	// clearing it is mined, not dug, and yields a little metal_ore.
	case "rubble_pile", "rubble_wall_h", "rubble_wall_vl", "rubble_wall_vr":
		return 3, 6, true
	}
	return 0, 0, false
}

// IsDepositTileName reports whether a tile type is a mineable resource deposit.
func IsDepositTileName(tileName string) bool {
	_, _, ok := depositRichnessRange(tileName)
	return ok
}

// DepositDrop returns the material blueprint a deposit tile yields when mined,
// or "" if the tile isn't a deposit.
func DepositDrop(tileName string) string {
	switch tileName {
	case "ore_deposit", "rubble_pile", "rubble_wall_h", "rubble_wall_vl", "rubble_wall_vr":
		return "metal_ore"
	case "crystal_vein":
		return "crystal"
	case "radioactive_ore":
		return "radioactive_material"
	}
	return ""
}

// RollDepositRichness rolls a deposit's total yield for the given tile type,
// biased toward the low end (squared roll) so the maximum is a rare find.
// Returns 0 for non-deposit tiles.
func RollDepositRichness(tileName string) int {
	min, max, ok := depositRichnessRange(tileName)
	if !ok {
		return 0
	}
	r := rand.Float64()
	return min + int(float64(max-min)*r*r)
}

// ResourceAmountAt returns the remaining yield of the deposit at (x,y,z), or 0
// if the tile isn't a tracked deposit.
func (l *Level) ResourceAmountAt(x, y, z int) int {
	if l.ResourceAmount == nil {
		return 0
	}
	return l.ResourceAmount[PackCoord(x, y, z)]
}

// SetResourceAmount records a deposit's remaining yield. An amount <= 0 clears
// the entry.
func (l *Level) SetResourceAmount(x, y, z, amount int) {
	if l.ResourceAmount == nil {
		l.ResourceAmount = make(map[TileCoord]int)
	}
	key := PackCoord(x, y, z)
	if amount <= 0 {
		delete(l.ResourceAmount, key)
		return
	}
	l.ResourceAmount[key] = amount
}

// ConsumeResource removes up to want units from the deposit at (x,y,z) and
// returns how many were actually extracted (clamped to what remains). The entry
// is deleted when it reaches zero. This is the single source of truth for a
// deposit's yield, so cancelling and re-issuing a mine order resumes from the
// depleted amount rather than re-rolling.
func (l *Level) ConsumeResource(x, y, z, want int) int {
	if l.ResourceAmount == nil || want <= 0 {
		return 0
	}
	key := PackCoord(x, y, z)
	remaining := l.ResourceAmount[key]
	if remaining <= 0 {
		return 0
	}
	if want > remaining {
		want = remaining
	}
	remaining -= want
	if remaining <= 0 {
		delete(l.ResourceAmount, key)
	} else {
		l.ResourceAmount[key] = remaining
	}
	return want
}
