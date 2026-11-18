package campaign

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// TestObserverCrossing measures the observer run's expedition on a medium
// galaxy in the abstract step (pregenerated_galaxy_plan.md, "Distances and
// fuel"): over 1000 days, its landings, the jumps it made, the farthest it
// got from its landfall against the far side of the galaxy (the farthest
// system from the landfall), the day it first landed within a quarter of
// that from the far side (a crossing), and its fuel. LP_CROSSING=1 runs it.
func TestObserverCrossing(t *testing.T) {
	if os.Getenv("LP_CROSSING") == "" {
		t.Skip("LP_CROSSING=1 measures the crossing")
	}
	useRepoData(t)
	var b strings.Builder
	fmt.Fprintf(&b, "%-5s %-14s %8s %6s %9s %8s %8s %6s\n", "seed", "shape", "landings", "jumps", "farthest", "farside", "crossed", "fuel")
	for seed := int64(1); seed <= 6; seed++ {
		c, err := GenerateCampaignOpts("Obs", seed, GenOptions{Factions: 2, Observer: true, Size: "medium"})
		if err != nil {
			t.Fatal(err)
		}
		cfg := genConfig()
		if _, err := c.AddExpeditionFleet(cfg.StartColonists, map[string]int{"metal_ore": 100, "crystal": 100}, cfg.StartFuel); err != nil {
			t.Fatal(err)
		}
		start := c.StartLocation()
		far := 0.0
		for _, l := range c.Systems() {
			far = math.Max(far, math.Hypot(l.X-start.X, l.Y-start.Y))
		}
		landings, jumps, crossed := 0, 0, -1
		farthest := 0.0
		prev := ""
		for day := 1; day <= 1000; day++ {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			f := c.Expedition()
			if f == nil {
				break
			}
			state := f.LandedID + ">" + f.ToID
			if state == prev {
				continue
			}
			prev = state
			if !f.Landed() {
				jumps++
				continue
			}
			landings++
			l := c.Locations[f.LandedID]
			d := math.Hypot(l.X-start.X, l.Y-start.Y)
			farthest = math.Max(farthest, d)
			if crossed < 0 && d >= 0.75*far {
				crossed = day
			}
		}
		f := c.Expedition()
		fuel := 0
		if f != nil {
			fuel = f.Presence.Fuel
		}
		cr := "no"
		if crossed >= 0 {
			cr = fmt.Sprintf("d%d", crossed)
		}
		fmt.Fprintf(&b, "%-5d %-14s %8d %6d %9.0f %8.0f %8s %6d\n", seed, c.Galaxy.Shape, landings, jumps, farthest, far, cr, fuel)
	}
	t.Log("\n" + b.String())
}

// TestPlayerRouteToTheGate measures a ship run from landfall to leaving on
// medium galaxies: the shortest route of hops of at most 60 from the
// landfall system to the controls world (its landings and fuel), then the
// gate and the leaving fuel. LP_CROSSING=1 runs it.
func TestPlayerRouteToTheGate(t *testing.T) {
	if os.Getenv("LP_CROSSING") == "" {
		t.Skip("LP_CROSSING=1 measures the run")
	}
	useRepoData(t)
	const hop = 60.0
	var b strings.Builder
	fmt.Fprintf(&b, "%-5s %-14s %8s %9s %10s %10s\n", "seed", "shape", "landings", "route", "gate hop", "fuel")
	for seed := int64(1); seed <= 6; seed++ {
		c, err := GenerateCampaignOpts("Run", seed, GenOptions{Size: "medium"})
		if err != nil {
			t.Fatal(err)
		}
		nodes := c.Systems()
		start, goal := -1, -1
		for i, l := range nodes {
			if l.ID == c.StartID {
				start = i
			}
			if l.ID == c.Galaxy.ControlsID {
				goal = i
			}
		}
		dist := make([]float64, len(nodes))
		hops := make([]int, len(nodes))
		done := make([]bool, len(nodes))
		for i := range dist {
			dist[i] = math.Inf(1)
		}
		dist[start] = 0
		for {
			u := -1
			for i := range nodes {
				if !done[i] && (u < 0 || dist[i] < dist[u]) {
					u = i
				}
			}
			if u < 0 || math.IsInf(dist[u], 1) {
				break
			}
			done[u] = true
			for v := range nodes {
				e := math.Hypot(nodes[u].X-nodes[v].X, nodes[u].Y-nodes[v].Y)
				if e <= hop && dist[u]+e < dist[v] {
					dist[v], hops[v] = dist[u]+e, hops[u]+1
				}
			}
		}
		gate, ctl := c.GateLocation(), c.ControlsLocation()
		gh := math.Hypot(gate.X-ctl.X, gate.Y-ctl.Y)
		first := float64(c.FuelCost(c.StartID))
		fuel := first + dist[goal] + gh + float64(LeaveFuel())
		fmt.Fprintf(&b, "%-5d %-14s %8d %9.0f %10.0f %10.0f\n", seed, c.Galaxy.Shape, hops[goal]+1, dist[goal], gh, fuel)
	}
	t.Log("\n" + b.String())
}
