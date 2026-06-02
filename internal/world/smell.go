package world

// SmellTag categorizes a scent. Define common tags as constants; arbitrary
// strings are accepted at the API boundary.
type SmellTag string

const (
	SmellTagColonist SmellTag = "colonist"
	SmellTagRot      SmellTag = "rot"
	SmellTagBlood    SmellTag = "blood"
)

// TileCoord is a packed 3D tile coordinate (x:16, y:16, z:16). Used as map
// keys for sparse spatial data — fast hash + equality vs. struct keys.
type TileCoord uint64

func PackCoord(x, y, z int) TileCoord {
	return TileCoord(uint64(uint16(x)) | uint64(uint16(y))<<16 | uint64(uint16(z))<<32)
}

func (c TileCoord) Unpack() (x, y, z int) {
	return int(int16(c)), int(int16(c >> 16)), int(int16(c >> 32))
}

// Scent constants. Tune to taste; eventually should become per-tag config.
const (
	// ScentDecay is the multiplicative loss per scent-tick, applied to all
	// tags. Scent-ticks run every ScentTickInterval rounds (see ScentSystem),
	// not every round — so persistence in rounds is roughly
	// ln(epsilon)/ln(decay) * interval. With decay=0.985 and interval=5 a
	// 1.0 deposit takes ~990 rounds (~100 player actions) to fade.
	ScentDecay float32 = 0.985

	// ScentTickInterval is the number of stepWorld rounds between scent
	// decay/diffusion passes. Decoupled from per-round updates so the
	// sparse-map iteration is amortized and trails persist over many turns.
	ScentTickInterval uint64 = 5
	// ScentDiffuse is the fraction of a tile's strength that flows to each
	// cardinal neighbor per diffusion pass. 4 * ScentDiffuse leaves the tile;
	// the rest stays. With 0.05 the tile retains 80%, neighbors get 5% each —
	// trails stay concentrated and decay drives persistence, not diffusion.
	ScentDiffuse float32 = 0.05
	// ScentEpsilon is the drop-below-this-value threshold for sparse cleanup.
	ScentEpsilon float32 = 0.05
)

// EmitScent adds amount of the given tag at (x,y,z). Existing strength at
// the tile is accumulated (not replaced). Sub-epsilon contributions are
// dropped to keep the map sparse.
func (l *Level) EmitScent(x, y, z int, tag SmellTag, amount float32) {
	if amount < ScentEpsilon {
		return
	}
	if l.SmellMap == nil {
		l.SmellMap = make(map[SmellTag]map[TileCoord]float32)
	}
	tagMap, ok := l.SmellMap[tag]
	if !ok {
		tagMap = make(map[TileCoord]float32)
		l.SmellMap[tag] = tagMap
		l.SmellTagOrder = append(l.SmellTagOrder, tag)
	}
	tagMap[PackCoord(x, y, z)] += amount
}

// DecayScent multiplies every entry of every tag by ScentDecay and drops
// sub-epsilon entries. Cheap — iterates only existing sparse entries.
func (l *Level) DecayScent() {
	for _, tagMap := range l.SmellMap {
		for k, v := range tagMap {
			nv := v * ScentDecay
			if nv < ScentEpsilon {
				delete(tagMap, k)
			} else {
				tagMap[k] = nv
			}
		}
	}
}

// DiffuseNextTag spreads the next tag in the rotation outward to cardinal
// neighbors. Sharded so only one tag is diffused per call — over N rounds
// (where N is the number of active tags) every tag gets one diffusion pass.
func (l *Level) DiffuseNextTag() {
	if len(l.SmellTagOrder) == 0 {
		return
	}
	tag := l.SmellTagOrder[l.SmellDiffuseIndex%len(l.SmellTagOrder)]
	l.SmellDiffuseIndex = (l.SmellDiffuseIndex + 1) % len(l.SmellTagOrder)

	src := l.SmellMap[tag]
	if len(src) == 0 {
		return
	}

	// Diffusion is additive — the source keeps its full strength and a
	// fraction is *radiated* to each cardinal neighbor. Decay (a separate
	// per-tick pass) is the only loss term. This is non-conservative (mass
	// can grow until decay balances it) but produces the trail behavior we
	// want: source tiles stay strong while a halo builds around them.
	next := make(map[TileCoord]float32, len(src))
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for coord, strength := range src {
		x, y, z := coord.Unpack()
		if strength >= ScentEpsilon {
			next[coord] += strength
		}
		contrib := strength * ScentDiffuse
		if contrib >= ScentEpsilon {
			for _, d := range dirs {
				next[PackCoord(x+d[0], y+d[1], z)] += contrib
			}
		}
	}
	// Sweep sub-epsilon entries in case multiple small contributions still
	// add up to noise.
	for k, v := range next {
		if v < ScentEpsilon {
			delete(next, k)
		}
	}
	l.SmellMap[tag] = next
}

// SmellAt returns the strength of a specific tag at one tile, or 0.
func (l *Level) SmellAt(x, y, z int, tag SmellTag) float32 {
	if l.SmellMap == nil {
		return 0
	}
	tagMap, ok := l.SmellMap[tag]
	if !ok {
		return 0
	}
	return tagMap[PackCoord(x, y, z)]
}

// StrongestSmellWithin returns the tag, position, and strength of the
// strongest scent within radius of (cx,cy,cz) on the same z-level. Returns
// empty/zero values if nothing is present above 0.
func (l *Level) StrongestSmellWithin(cx, cy, cz, radius int) (SmellTag, int, int, int, float32) {
	var bestTag SmellTag
	var bestX, bestY, bestZ int
	var bestStrength float32
	r2 := radius * radius
	for tag, tagMap := range l.SmellMap {
		for coord, strength := range tagMap {
			x, y, z := coord.Unpack()
			if z != cz {
				continue
			}
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy > r2 {
				continue
			}
			if strength > bestStrength {
				bestTag = tag
				bestX, bestY, bestZ = x, y, z
				bestStrength = strength
			}
		}
	}
	return bestTag, bestX, bestY, bestZ, bestStrength
}
