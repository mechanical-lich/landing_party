package campaign

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/lore_golden_sites.json from the current generator")

// goldenSite is what generation decided about a location apart from its
// history: its map, position and name.
type goldenSite struct {
	ID, Name, Kind, Map string
	X, Y                float64
}

func goldenRun(t *testing.T, seed int64) []goldenSite {
	t.Helper()
	c := genCampaign(t, seed)
	var out []goldenSite
	for _, l := range c.Locations {
		out = append(out, goldenSite{ID: l.ID, Name: l.Name, Kind: l.Kind, Map: l.MapID,
			X: float64(int(l.X*1000)) / 1000, Y: float64(int(l.Y*1000)) / 1000})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// A seed's galaxy keeps its maps, positions and names: a change to
// generation that moves them is made on purpose (-update-golden), never by
// accident. Re-pinned for the pregenerated galaxy (plan step 1).
func TestLoreHistoryKeepsSeedsMapsPositionsNames(t *testing.T) {
	useRepoData(t)
	got := map[string][]goldenSite{}
	for _, seed := range []int64{1, 7, 42, 1977, 31337} {
		got[fmt.Sprint(seed)] = goldenRun(t, seed)
	}
	const path = "testdata/lore_golden_sites.json"
	if *updateGolden {
		b, _ := json.MarshalIndent(got, "", " ")
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string][]goldenSite
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	for seed, sites := range want {
		g := got[seed]
		if len(g) != len(sites) {
			t.Errorf("seed %s: %d locations, golden has %d", seed, len(g), len(sites))
			continue
		}
		for i := range sites {
			if g[i] != sites[i] {
				t.Errorf("seed %s: %+v, golden %+v", seed, g[i], sites[i])
			}
		}
	}
}
