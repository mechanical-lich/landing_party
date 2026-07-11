package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

func TestSpawnTargets(t *testing.T) {
	level := world.NewLevel(8, 8, 10)
	level.AllocTerrain()
	level.SurfaceZ = 5

	// underground cavern / mountain cavern / underground rock / mountain rock
	level.SetTerrainKind(1, 1, 2, world.TKCavern)
	level.SetTerrainKind(2, 2, 7, world.TKCavern)
	level.SetTerrainKind(3, 3, 3, world.TKUnderground)
	level.SetTerrainKind(4, 4, 8, world.TKUnderground)

	has := func(ts [][3]int, x, y, z int) bool {
		for _, tt := range ts {
			if tt == [3]int{x, y, z} {
				return true
			}
		}
		return false
	}

	// Open caverns.
	if c := spawnTargets(level, false, "any"); !has(c, 1, 1, 2) || !has(c, 2, 2, 7) || has(c, 3, 3, 3) {
		t.Errorf("cavern/any wrong: %v", c)
	}
	if c := spawnTargets(level, false, "mountain"); !has(c, 2, 2, 7) || has(c, 1, 1, 2) {
		t.Errorf("cavern/mountain wrong: %v", c)
	}
	if c := spawnTargets(level, false, "underground"); !has(c, 1, 1, 2) || has(c, 2, 2, 7) {
		t.Errorf("cavern/underground wrong: %v", c)
	}

	// Solid rock (buried).
	if r := spawnTargets(level, true, "any"); !has(r, 3, 3, 3) || !has(r, 4, 4, 8) || has(r, 1, 1, 2) {
		t.Errorf("rock/any wrong: %v", r)
	}
	if r := spawnTargets(level, true, "mountain"); !has(r, 4, 4, 8) || has(r, 3, 3, 3) {
		t.Errorf("rock/mountain wrong: %v", r)
	}
	if r := spawnTargets(level, true, "underground"); !has(r, 3, 3, 3) || has(r, 4, 4, 8) {
		t.Errorf("rock/underground wrong: %v", r)
	}
}
