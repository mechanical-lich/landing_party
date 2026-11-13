package game

import (
	"log"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// Ruins on departure (phase D4; docs/developer/factions.md, "Ruins"). When
// a fleet faction's ship lifts off, the campaign records a FactionRuin on
// the site it left (liftOff). The level turns the camp into a ruin once: at
// once when the level is loaded (the player watched it leave), else at its
// next build. The ruin is an ordinary stamp to the lore system: its
// footprint joins StampFootprints (the budget and later stamps keep clear),
// the departure's Legends event gets an origin at the camp, and a datapad
// there holds that line. Whatever the camp still held is gone: the
// stockpile left with the ship.

// ruinDamage is ruin_pass's damage for a departed camp: broken, not razed.
const ruinDamage = 2

// stampRuins ruins every camp on loc not yet ruined on level.
func stampRuins(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	if loc == nil || level == nil {
		return
	}
	fd := campaign.LoadFactionData()
	for i := range loc.Ruins {
		r := &loc.Ruins[i]
		if r.Stamped {
			continue
		}
		if d := fd.Doctrine(r.Doctrine); d != nil && d.Settlement != nil {
			stampRuin(c, level, loc, d, r)
		}
		r.Stamped = true
	}
}

// stampRuin ruins one departed camp: the camp built first if the ship came
// and went while the level was frozen, its crew and hold cleared out, then
// ruin_pass over it, the departure's origin and a datapad at its centre.
func stampRuin(c *campaign.Campaign, level *world.Level, loc *campaign.Location, d *campaign.Doctrine, r *campaign.FactionRuin) {
	p := &campaign.FactionPresence{ID: r.Presence, Faction: r.Faction, Doctrine: r.Doctrine, Seed: r.Seed}
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] ruin of %s on %s: no room", r.Presence, loc.Name)
		return
	}
	name := factionSettlementName(p, loc.ID)
	w, h := campSize(d)
	if isGreyishDoctrine(d) {
		// The hull went with the ship: a landing scar, its modules packed
		// and its ground structures wrecked (phase E4).
		stampGreyScar(level, d, r, p, name, cx, cy, cz)
		finishRuin(level, r, cx, cy, cz, w, h)
		return
	}
	if !r.CampBuilt {
		if d.Settlement.Camp != nil {
			buildStampedCamp(level, d, p, name, cx, cy, cz)
		} else {
			buildFactionCamp(level, d, p, name, cx, cy, cz)
		}
	}
	// The crew went aboard and the hold with them.
	for _, e := range settlementWorkers(level, name) {
		level.RemoveEntity(e)
	}
	emptySettlementStorage(level, name)
	wall, floor := "bunker_wall", "bunker_floor"
	if d.Settlement.Camp != nil {
		wall, floor = "hull_wall", "hull_floor"
	}
	x0, y0 := cx-w/2, cy-h/2
	ctx := &setupContext{Level: level, rng: rand.New(rand.NewSource(r.Seed*7 + int64(r.Day)))}
	top := ctx.topParams()
	top["damage"], top["wall"], top["floor"] = float64(ruinDamage), wall, floor
	if err := runGenStructure(ctx, "ruin_pass", x0, y0, w, h); err != nil {
		log.Printf("[faction] ruin_pass for %s: %v", r.Presence, err)
	}
	finishRuin(level, r, cx, cy, cz, w, h)
	log.Printf("[faction] %s's camp on %s left as a ruin", r.Presence, loc.Name)
}

// finishRuin records a ruin's footprint (the lore budget keeps clear of
// it) and puts the departure's origin and a datapad holding it at the
// centre.
func finishRuin(level *world.Level, r *campaign.FactionRuin, cx, cy, cz, w, h int) {
	x0, y0 := cx-w/2, cy-h/2
	level.StampFootprints = append(level.StampFootprints, [6]int{x0, y0, w, h, cx, cy})
	if r.EventID != "" {
		generation.SetOrigin(level, r.EventID, cx, cy, cz)
		x, y, z := freeLandingTile(level, cx, cy, cz)
		if pad, err := factory.Create("datapad", x, y, z); err == nil {
			pad.AddComponent(&components.DatapadComponent{EventID: r.EventID})
			level.AddEntity(pad)
		}
	}
}

// campSize is a doctrine's camp footprint: its camp block's, else the
// bunker room's (the first stage's, at least 9x7).
func campSize(d *campaign.Doctrine) (int, int) {
	if cp := d.Settlement.Camp; cp != nil {
		return cp.W, cp.H
	}
	w, h := 9, 7
	if len(d.Stages) > 0 {
		w, h = max(d.Stages[0].W, w), max(d.Stages[0].H, h)
	}
	return w, h
}

// ruinDatapads are the datapads on level holding a faction event, for tests.
func ruinDatapads(level *world.Level, eventID string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range level.Entities {
		if e != nil && e.HasComponent(components.Datapad) &&
			e.GetComponent(components.Datapad).(*components.DatapadComponent).EventID == eventID {
			out = append(out, e)
		}
	}
	return out
}
