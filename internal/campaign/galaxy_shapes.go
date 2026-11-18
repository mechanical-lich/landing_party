package campaign

import (
	"math"
	"math/rand"
)

// A galaxy shape is a density over the unit disc (x, y in [-1, 1], scaled
// by the size's radius when placed): the placer samples points uniformly in
// the disc and keeps one with the density's probability (galaxy.go). Each
// shape rolls its own parameters (arm count, rotation, clump centres) from
// the galaxy stream when it is built, so two spirals differ. Adding a shape
// is a builder here and a row in generation.json's galaxy.shapes.
type densityFunc func(x, y float64) float64

// shapeBuilders maps a galaxy.shapes id to its builder.
var shapeBuilders = map[string]func(rng *rand.Rand) densityFunc{
	"spiral":        spiralDensity,
	"barred_spiral": barredSpiralDensity,
	"elliptical":    ellipticalDensity,
	"lenticular":    lenticularDensity,
	"ring":          ringDensity,
	"irregular":     irregularDensity,
}

// shapeFloor keeps a thin scatter of systems everywhere inside the disc, so
// no shape leaves a landfall with nothing within reach.
const shapeFloor = 0.015

func gauss(d, w float64) float64 { return math.Exp(-(d * d) / (w * w)) }

// angDist is the shortest angle between a and b.
func angDist(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 2*math.Pi)
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	return d
}

// spiralDensity: two to four logarithmic arms winding out from a dense core.
func spiralDensity(rng *rand.Rand) densityFunc {
	arms := 2 + rng.Intn(3)
	rot := rng.Float64() * 2 * math.Pi
	wind := 2.2 + rng.Float64()*1.6 // radians of winding per unit of ln r
	sign := 1.0
	if rng.Intn(2) == 0 {
		sign = -1
	}
	return func(x, y float64) float64 {
		r := math.Hypot(x, y)
		if r > 1 {
			return 0
		}
		th := math.Atan2(y, x)
		core := gauss(r, 0.16)
		best := math.Pi
		for k := 0; k < arms; k++ {
			arm := rot + float64(k)*2*math.Pi/float64(arms) + sign*wind*math.Log(math.Max(r, 0.05))
			best = math.Min(best, angDist(th, arm))
		}
		// The arm's width in distance, not angle, so it stays even outward.
		arm := gauss(best*r, 0.07+0.05*r) * (1 - 0.55*r) * smooth(r, 0.12)
		return math.Min(1, core+arm+shapeFloor)
	}
}

// barredSpiralDensity: a straight bar through the core with an arm off each end.
func barredSpiralDensity(rng *rand.Rand) densityFunc {
	rot := rng.Float64() * 2 * math.Pi
	bar := 0.28 + rng.Float64()*0.12
	wind := 2.0 + rng.Float64()*1.2
	sign := 1.0
	if rng.Intn(2) == 0 {
		sign = -1
	}
	cs, sn := math.Cos(-rot), math.Sin(-rot)
	return func(x, y float64) float64 {
		r := math.Hypot(x, y)
		if r > 1 {
			return 0
		}
		bx, by := x*cs-y*sn, x*sn+y*cs // in the bar's frame
		d := 0.0
		if ax := math.Abs(bx); ax > bar {
			d = ax - bar
		}
		barD := gauss(math.Hypot(d, by), 0.07) * 0.85
		core := gauss(r, 0.12)
		armD := 0.0
		if r > bar*0.8 {
			th := math.Atan2(by, bx)
			best := math.Pi
			for k := 0; k < 2; k++ {
				arm := float64(k)*math.Pi + sign*wind*math.Log(r/bar)
				best = math.Min(best, angDist(th, arm))
			}
			armD = gauss(best*r, 0.08+0.04*r) * (1 - 0.5*r)
		}
		return math.Min(1, core+barD+armD+shapeFloor)
	}
}

// ellipticalDensity: one smooth blob, dense in the middle, thinning outward.
func ellipticalDensity(rng *rand.Rand) densityFunc {
	rot := rng.Float64() * math.Pi
	axis := 0.55 + rng.Float64()*0.35 // minor/major
	cs, sn := math.Cos(-rot), math.Sin(-rot)
	return func(x, y float64) float64 {
		if math.Hypot(x, y) > 1 {
			return 0
		}
		ex, ey := x*cs-y*sn, (x*sn+y*cs)/axis
		e := math.Hypot(ex, ey)
		if e > 1 {
			return shapeFloor * 0.5
		}
		return math.Min(1, gauss(e, 0.5)+shapeFloor)
	}
}

// lenticularDensity: a disc with a bright core and no arms.
func lenticularDensity(rng *rand.Rand) densityFunc {
	edge := 0.8 + rng.Float64()*0.15
	return func(x, y float64) float64 {
		r := math.Hypot(x, y)
		if r > 1 {
			return 0
		}
		core := gauss(r, 0.18)
		disc := 0.3 * (1 - smooth(r, edge))
		return math.Min(1, core+disc+shapeFloor)
	}
}

// ringDensity: a core, a gap, and a ring of systems round it.
func ringDensity(rng *rand.Rand) densityFunc {
	ring := 0.68 + rng.Float64()*0.14
	width := 0.07 + rng.Float64()*0.04
	return func(x, y float64) float64 {
		r := math.Hypot(x, y)
		if r > 1 {
			return 0
		}
		core := 0.9 * gauss(r, 0.13)
		band := gauss(r-ring, width)
		return math.Min(1, core+band+shapeFloor*0.5)
	}
}

// irregularDensity: a few lopsided clumps with no centre.
func irregularDensity(rng *rand.Rand) densityFunc {
	type clump struct{ x, y, w, h, s float64 }
	n := 3 + rng.Intn(3)
	cl := make([]clump, n)
	for i := range cl {
		a := rng.Float64() * 2 * math.Pi
		r := 0.2 + rng.Float64()*0.5
		cl[i] = clump{x: math.Cos(a) * r, y: math.Sin(a) * r,
			w: 0.12 + rng.Float64()*0.14, h: 0.12 + rng.Float64()*0.14, s: 0.5 + rng.Float64()*0.5}
	}
	return func(x, y float64) float64 {
		if math.Hypot(x, y) > 1 {
			return 0
		}
		d := 0.0
		for _, c := range cl {
			dx, dy := (x-c.x)/c.w, (y-c.y)/c.h
			d += c.s * math.Exp(-(dx*dx + dy*dy))
		}
		return math.Min(1, d+shapeFloor)
	}
}

// smooth is 0 below a and climbs to 1 a little past it (a soft step).
func smooth(r, a float64) float64 {
	t := (r - a*0.6) / (a * 0.8)
	switch {
	case t <= 0:
		return 0
	case t >= 1:
		return 1
	}
	return t * t * (3 - 2*t)
}
