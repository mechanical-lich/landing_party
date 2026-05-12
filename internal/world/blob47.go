package world

import "github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"

// blob47MaskFor computes the pruned 8-direction neighbor mask of the cell's
// Middle slot for debug overlay purposes. Mirrors what rllayered's
// ResolveVariant does for AutoTileBlob47.
func blob47MaskFor(level *Level, t *Tile) uint8 {
	if t == nil || t.Middle.IsEmpty() {
		return 0
	}
	x, y, z := t.Coords()
	def := TileDefinitions[t.Middle.Type]
	isAutoTile := def.AutoTile != rllayered.AutoTileNone
	sameType := func(nx, ny, nz int) bool {
		n := level.GetTilePtr(nx, ny, nz)
		if n == nil || n.Middle.IsEmpty() {
			return false
		}
		if isAutoTile {
			return n.Middle.Type == t.Middle.Type
		}
		return n.Middle.Type == t.Middle.Type && n.Middle.Variant == t.Middle.Variant
	}
	var m uint8
	if sameType(x, y-1, z) {
		m |= rllayered.BlobBitN
	}
	if sameType(x, y+1, z) {
		m |= rllayered.BlobBitS
	}
	if sameType(x-1, y, z) {
		m |= rllayered.BlobBitW
	}
	if sameType(x+1, y, z) {
		m |= rllayered.BlobBitE
	}
	if sameType(x+1, y-1, z) {
		m |= rllayered.BlobBitNE
	}
	if sameType(x-1, y-1, z) {
		m |= rllayered.BlobBitNW
	}
	if sameType(x+1, y+1, z) {
		m |= rllayered.BlobBitSE
	}
	if sameType(x-1, y+1, z) {
		m |= rllayered.BlobBitSW
	}
	return rllayered.PruneBlobMask(m)
}
