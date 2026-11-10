package generation

import (
	"sort"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// The conversion overlay (phase C2; docs/developer/factions.md,
// "ConvertAction"): a presence's Conversion share applied to its level as a
// patch radiating from its centre, the abstract mirror of its workers'
// ConvertActions. ApplyConversion lays the conversion floor on the share of
// the convertible ground within the radius nearest the centre (ties broken
// by a hash of the seed and the tile, so the edge is ragged but the same
// every time); ConversionShare reads the share back for reconcile.

// conversionTiles is the convertible ground within r of (cx, cy) on z
// (a floor and no solid middle; tile itself counts), nearest first, ties by
// the seeded hash.
func conversionTiles(level *world.Level, cx, cy, cz, r int, seed int64) [][2]int {
	type cand struct {
		x, y int
		key  uint64
	}
	var cands []cand
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			x, y := cx+dx, cy+dy
			ti, ok := level.GetTileAt(x, y, cz).(*world.Tile)
			if !ok || ti == nil || ti.Floor.IsEmpty() {
				continue
			}
			if !ti.Middle.IsEmpty() && world.TileDefinitions[ti.Middle.Type].Solid {
				continue
			}
			d2 := uint64(dx*dx + dy*dy)
			cands = append(cands, cand{x, y, d2<<32 | uint64(mix(seed, x, y)&0xffffffff)})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].key < cands[j].key })
	out := make([][2]int, len(cands))
	for i, c := range cands {
		out[i] = [2]int{c.x, c.y}
	}
	return out
}

func mix(seed int64, x, y int) uint64 {
	h := uint64(seed)*0x9E3779B97F4A7C15 ^ uint64(x)*0xBF58476D1CE4E5B9 ^ uint64(y)*0x94D049BB133111EB
	h ^= h >> 31
	h *= 0xD6E8FEB86659FD93
	h ^= h >> 32
	return h
}

// ApplyConversion brings the convertible ground within r of (cx, cy, cz)
// that is tile up to share (0..1), laying the shortfall nearest first.
// Ground already converted counts wherever it lies (a live director lays by
// reach from the centre and its converter nodes, not nearest first), so a
// thaw of a recorded share adds nothing and the share does not ratchet up
// with every freeze and thaw. Idempotent; it only adds. It returns the tiles
// converted now.
func ApplyConversion(level *world.Level, cx, cy, cz, r int, tile string, share float64, seed int64) int {
	if tile == "" || share <= 0 || r <= 0 {
		return 0
	}
	tiles := conversionTiles(level, cx, cy, cz, r, seed)
	n := min(int(share*float64(len(tiles))+0.5), len(tiles))
	have := 0
	for _, t := range tiles {
		if world.TileDefinitions[level.GetTileAt(t[0], t[1], cz).(*world.Tile).Floor.Type].Name == tile {
			have++
		}
	}
	done := 0
	for _, t := range tiles {
		if have+done >= n {
			break
		}
		ti := level.GetTileAt(t[0], t[1], cz).(*world.Tile)
		if world.TileDefinitions[ti.Floor.Type].Name == tile {
			continue
		}
		level.SetFloor(t[0], t[1], cz, tile, world.TileVariantAt(tile, t[0], t[1], cz))
		level.InvalidateSunColumn(t[0], t[1])
		done++
	}
	return done
}

// ConversionShare is the share of the convertible ground within r of (cx,
// cy, cz) that is tile.
func ConversionShare(level *world.Level, cx, cy, cz, r int, tile string) float64 {
	tiles := conversionTiles(level, cx, cy, cz, r, 0)
	if len(tiles) == 0 || tile == "" {
		return 0
	}
	n := 0
	for _, t := range tiles {
		ti := level.GetTileAt(t[0], t[1], cz).(*world.Tile)
		if world.TileDefinitions[ti.Floor.Type].Name == tile {
			n++
		}
	}
	return float64(n) / float64(len(tiles))
}

// ConversionCounts splits the convertible ground within r of (cx, cy, cz) at
// inner: inside is the tiles within inner of the centre, beyond the tiles
// past it that are tile, total every tile within r. A settlement whose camp
// is laid with its own floor (the hive's chamber of resin) measures its
// creep as beyond / total.
func ConversionCounts(level *world.Level, cx, cy, cz, r, inner int, tile string) (inside, beyond, total int) {
	tiles := conversionTiles(level, cx, cy, cz, r, 0)
	for _, t := range tiles {
		dx, dy := t[0]-cx, t[1]-cy
		if dx*dx+dy*dy <= inner*inner+inner { // carve_round_room's circle
			inside++
			continue
		}
		ti := level.GetTileAt(t[0], t[1], cz).(*world.Tile)
		if world.TileDefinitions[ti.Floor.Type].Name == tile {
			beyond++
		}
	}
	return inside, beyond, len(tiles)
}
