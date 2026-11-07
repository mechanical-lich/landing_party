package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// contentHistoryMaps are the maps with a history, sorted.
func contentHistoryMaps() []string {
	var out []string
	for _, m := range mapdef.All() {
		if m.Present != nil {
			out = append(out, m.ID)
		}
	}
	return out
}

// contentBuildTerrain builds a charted site's level the way MainState.generate
// does (the map's, the scenario's and the history's features, origins and
// keep-outs) but without setup scripts, structure scripts or a MainState: the
// stamps' footprints are recorded, nothing is painted.
func contentBuildTerrain(t *testing.T, lib *history.Library, h *history.History, site *history.SiteHistory) *world.Level {
	t.Helper()
	md := mapdef.ByID(site.Map)
	w, hh, z := md.Size.Roll(site.Seed)
	ev := h.Evidence(lib, site)
	opts := generation.BuildWorldOptions{
		Width: w, Height: hh, Depth: z, Seed: site.Seed,
		Terrain: md.Terrain, TerrainParams: md.TerrainParams,
		BiomeMapType: md.BiomeMap.Type, BiomeMapScale: md.BiomeMap.Scale,
		BiomeLatitudeWeight: md.BiomeMap.LatitudeWeight, BiomeIDs: md.BiomeMap.Biomes,
		BiomeSingle: md.BiomeMap.Single, ReferenceArea: md.Size.MidArea(),
		SkipBiomeFeatures: md.SkipBiomeFeatures,
		Origins:           ev.Origins, ChapterEvent: ev.Chapter, OriginKeepOut: ev.KeepOut,
		Flags: h.LoreFlags(lib, site),
	}
	blocks := append([]mapdef.FeatureBlock(nil), md.Features...)
	if sc := scenario.ByID(site.Scenario); sc != nil {
		blocks = append(blocks, sc.Features...)
	}
	blocks = append(blocks, ev.Features...)
	for _, fb := range blocks {
		opts.Features = append(opts.Features, generation.FeatureSpec{
			Kind: fb.Kind, Count: fb.Count, CountMax: fb.CountMax, Biome: fb.Biome,
			InRegion: fb.InRegion, Jitter: fb.Jitter, MinZ: fb.MinZ, MaxZ: fb.MaxZ,
			Params: fb.Params, At: fb.At,
		})
	}
	level, err := generation.BuildWorld(opts)
	if err != nil {
		t.Fatalf("%s seed %d: %v", site.Map, site.Seed, err)
	}
	return level
}

// No history stamp overlaps the landing reserve or stands within the stamp
// spacing of another, across 50 charted sites per map; every hazard origin
// keeps its keep-out where the map is big enough to allow it.
func TestContentStampsKeepClear(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 400 sites")
	}
	loreLoadData(t)
	generation.SetStructureRunner(func(*world.Level, string, int, int, int, int, map[string]any, int64) error { return nil })
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	b := generation.Budget()
	maps := contentHistoryMaps()
	stamps := 0
	for seed := int64(1); seed <= 50; seed++ {
		h := history.New(seed * 7919)
		for i, m := range maps {
			site, err := h.BindSite(lib, m, h.Seed+int64(i+1)*131)
			if err != nil {
				t.Fatal(err)
			}
			level := contentBuildTerrain(t, lib, h, site)
			lx, ly, _, ok := generation.Landing(level)
			if !ok {
				t.Errorf("%s seed %d: no landing reserve", m, site.Seed)
				continue
			}
			half := b.LandingHalf
			spacing := b.For(level.GetWidth()).Spacing
			fs := level.StampFootprints
			for j, f := range fs {
				stamps++
				if f[0] <= lx+half && f[0]+f[2]-1 >= lx-half && f[1] <= ly+half && f[1]+f[3]-1 >= ly-half {
					t.Errorf("%s seed %d: stamp %v overlaps the landing reserve at (%d,%d)", m, site.Seed, f, lx, ly)
				}
				for _, g := range fs[:j] {
					if f[4] >= 0 && f[4] == g[4] && f[5] == g[5] {
						continue // two stamps of one origin (a meltdown restamps its reactor)
					}
					gx := max(g[0]-(f[0]+f[2]), f[0]-(g[0]+g[2]))
					gy := max(g[1]-(f[1]+f[3]), f[1]-(g[1]+g[3]))
					if max(gx, gy) < spacing {
						t.Errorf("%s seed %d: stamps %v and %v are %d apart, want %d", m, site.Seed, f, g, max(gx, gy), spacing)
					}
				}
			}
		}
	}
	if stamps == 0 {
		t.Error("no stamps were placed")
	}
}

// The colony lands in the landing reserve, a real build per map: generation
// picked it before any feature, and the plaza search finds it.
func TestContentLandingInReserve(t *testing.T) {
	loreLoadData(t)
	half := generation.Budget().LandingHalf
	for _, m := range contentHistoryMaps() {
		md := mapdef.ByID(m)
		ms, _, _, _ := loreBuild(t, loreSite{Map: m, Scenario: md.BaseScenario, Seed: 97})
		lx, ly, _, ok := generation.Landing(ms.level)
		if !ok || ms.MainSettlement == nil {
			t.Errorf("%s: landing reserve %v, settlement %v", m, ok, ms.MainSettlement != nil)
			continue
		}
		cx, cy := ms.MainSettlement.CenterX, ms.MainSettlement.CenterY
		if max(abs(cx-lx), abs(cy-ly)) > half {
			t.Errorf("%s: landed at (%d,%d), reserve at (%d,%d)", m, cx, cy, lx, ly)
		}
	}
}
