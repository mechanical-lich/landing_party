package game

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// Hostility seeding (phase C2; the plan's finding 3; docs/developer/
// factions.md, "Hostility"). A faction's people are spawned hostile or
// neutral by its doctrine and the campaign's diplomacy: the stance marks
// the faction for the player's colonists (components.SetFactionHostile:
// hostile workers become attack targets), and a unit driven by a script
// (a guard) has its "hostility" var seeded before its first turn from the
// doctrine's aggression (1, below every hunting threshold, while neutral),
// so the humanoid script does not roll its own. Creatures spawned by
// scenarios and the hive keep their own rules.

// syncFactionHostility marks every faction doctrine's stance for the
// player's colonists from the campaign (nil: each doctrine's own).
func syncFactionHostility(c *campaign.Campaign) {
	components.ResetFactionHostility()
	fd := campaign.LoadFactionData()
	player := playerFactionKey(c)
	keys := map[string]bool{}
	for key := range fd.Doctrines {
		keys[key] = true
	}
	if c != nil {
		for _, f := range c.Factions() { // every faction instance (rival_selection_plan.md)
			keys[f.Key] = true
		}
		// and every faction the campaign holds or has a stance with, table or
		// not (a faction's key is not its doctrine's: the rival colony is
		// "mirror", running the colony's rival view)
		for k := range c.Diplomacy {
			keys[k] = true
		}
		for _, loc := range c.Locations {
			for i := range loc.Factions {
				keys[loc.Factions[i].Faction] = true
			}
		}
		for i := range c.Fleets {
			keys[c.Fleets[i].Presence.Faction] = true
		}
	}
	for key := range keys {
		// A rival's faction key is its doctrine's; only the player's own
		// faction is skipped, never a rival that runs the player's doctrine
		// (a hive player still meets rival hives).
		if key != player {
			components.SetFactionHostile(key, c.Stance(key) == campaign.StanceHostile)
		}
	}
}

// seedHostility applies a faction's stance to one of its people as it is
// spawned: a worker defends itself; a scripted unit's hostility var is set
// from the doctrine's aggression when hostile, 1 when neutral.
func seedHostility(e *ecs.Entity, d *campaign.Doctrine, hostile bool) {
	if e.HasComponent(components.Worker) {
		e.GetComponent(components.Worker).(*components.WorkerComponent).SelfDefend = true
	}
	if e.HasComponent(components.ScriptedAI) {
		sc := e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
		if sc.Vars == nil {
			sc.Vars = map[string]any{}
		}
		sc.Vars["hostility"] = float64(scriptHostility(d, hostile))
	}
}

// scriptHostility is the humanoid script's hostility for a doctrine's unit:
// its aggression (1..10) when hostile, 1 (passive) when neutral.
func scriptHostility(d *campaign.Doctrine, hostile bool) int {
	if !hostile {
		return 1
	}
	return min(max(d.Aggression, 1), 10)
}

// contactFactions is the player landing on loc (a colonist beamed down):
// every faction holding it with a settlement that is neutral turns hostile
// (the plan's "Mirror colony diplomacy"), on every loaded level at once.
func (wm *WorldManager) contactFactions(loc *campaign.Location) {
	c := wm.Campaign
	if c == nil || loc == nil {
		return
	}
	for _, f := range c.HeldBy(loc) {
		if c.Stance(f) != campaign.StanceNeutral {
			continue
		}
		c.SetStance(f, campaign.StanceHostile, loc)
		components.SetFactionHostile(f, true)
		d := campaign.LoadFactionData().Doctrine(c.DoctrineOf(f))
		name := f
		if d != nil {
			name = d.MorphAt(loc).Singular
			for _, ms := range wm.loaded {
				if ms == nil || ms.level == nil {
					continue
				}
				for _, e := range ms.level.Entities {
					if e != nil && e.HasComponent(rlcomponents.Description) &&
						e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction == f {
						seedHostility(e, d, true)
					}
				}
			}
		}
		log.Printf("[faction] %s turned hostile: the player landed on %s", f, loc.Name)
		message.PostMessage("Alert", fmt.Sprintf("The %s on %s takes your landing as a threat. It is hostile now.", name, loc.Name))
	}
}

// playerFactionKey is the faction key of the player's own people: the player
// presence's faction (the colony's is its doctrine key, "colony"), else the
// default doctrine's.
// playerPeopleFaction is the faction the player's own people carry on a
// level (systems.PlayerFaction): the player's faction key in a ship run
// ("colony", or "player" for the greys); "colony" otherwise, so a spreading
// race's bodies and an observer run open the doors they always did.
func playerPeopleFaction(c *campaign.Campaign) string {
	if c == nil || c.Observer || c.SpreadRun() {
		return campaign.DefaultPlayerDoctrine
	}
	return playerFactionKey(c)
}

func playerFactionKey(c *campaign.Campaign) string {
	if c != nil && c.Player != nil && c.Player.Faction != "" {
		return c.Player.Faction
	}
	return campaign.DefaultPlayerDoctrine
}

// setFactionRelation points the bodies' faction relation at c's groups
// (nil: any two different factions are hostile, as before groups).
func setFactionRelation(c *campaign.Campaign) {
	if c == nil {
		components.FactionsHostile, components.FriendlyToPlayer = nil, nil
		return
	}
	components.FactionsHostile = c.Hostile
	components.FriendlyToPlayer = func(f string) bool {
		// a rival faction in the player's group (never the player's own)
		return c.Player != nil && f != c.Player.Faction && c.FactionOf(f) != nil && c.SameGroup(f, c.Player.Faction)
	}
}
