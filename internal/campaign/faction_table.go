package campaign

import (
	"fmt"
	"sort"
)

// Faction instances (rival_selection_plan.md, step 1). A campaign's factions
// are a table of instances, each running a doctrine: the faction key is
// opaque (never parsed for a doctrine), so two hives, or two grey factions,
// are two factions with their own presences, stance, mothership and
// director, while one faction per doctrine keeps the doctrine key as its
// faction key and plays exactly as before. The player and the observer
// run's expedition are rows of the same table.

// Faction is one faction instance.
type Faction struct {
	Key      string `json:"key"`
	Doctrine string `json:"doctrine"`
	// Name is shown for it ("" is the doctrine's own naming: one faction of
	// its doctrine); Badge its Star Map colour (zero: the doctrine's).
	Name  string `json:"name,omitempty"`
	Badge [3]int `json:"badge,omitempty"`
	// Slot is the rival box it came from (1 and up), 0 for the player.
	Slot int `json:"slot,omitempty"`
	// Group decides who fights whom: same group friendly, different group
	// hostile, whatever the doctrines (Campaign.Hostile).
	Group int `json:"group,omitempty"`
	// Ordinal is its place among the factions of its doctrine (2, 3...; 0
	// the first): the word that tells two of a kind apart while a hidden
	// identity keeps the doctrine's name back ("Xenos 2").
	Ordinal int `json:"ordinal,omitempty"`
	// GateKnown: its ships have found where the gate is (gate.go).
	GateKnown bool `json:"gate_known,omitempty"`
}

// PlayerGroup is the player's group; rivals start at 2, each its own.
const PlayerGroup = 1

// FactionOf is key's row (nil: no such faction in the table).
func (c *Campaign) FactionOf(key string) *Faction {
	if c == nil || c.FactionTable == nil {
		return nil
	}
	return c.FactionTable[key]
}

// DoctrineOf is the doctrine a faction key runs: its row's, else the
// player's (its key), the observer expedition's colony, else the key itself
// (a scenario creature's baked faction, a save before the table).
func (c *Campaign) DoctrineOf(key string) string {
	if f := c.FactionOf(key); f != nil && f.Doctrine != "" {
		return f.Doctrine
	}
	if c != nil && c.Player != nil && key == c.Player.Faction && c.Player.Doctrine != "" {
		return c.Player.Doctrine
	}
	if key == ExpeditionFaction {
		return DefaultPlayerDoctrine
	}
	return key
}

// GroupOf is key's group (0: a faction with no row, its own side alone).
func (c *Campaign) GroupOf(key string) int {
	if f := c.FactionOf(key); f != nil {
		return f.Group
	}
	return 0
}

// SameGroup reports two faction keys on one side: the same faction, or two
// rows sharing a group.
func (c *Campaign) SameGroup(a, b string) bool {
	if a == b {
		return true
	}
	ga, gb := c.GroupOf(a), c.GroupOf(b)
	return ga != 0 && ga == gb
}

// Hostile reports whether two factions' bodies fight where they meet: any
// two not on one side (rival_selection_plan.md, "Groups"). Towards the
// player, when is the doctrine's rule (Stance); whether is this.
func (c *Campaign) Hostile(a, b string) bool { return !c.SameGroup(a, b) }

// nextGroup is the lowest rival group no row has.
func (c *Campaign) nextGroup() int {
	used := map[int]bool{}
	for _, f := range c.FactionTable {
		used[f.Group] = true
	}
	g := PlayerGroup + 1
	for used[g] {
		g++
	}
	return g
}

// newFactionKey is a fresh key for doctrine: the doctrine key while no
// faction has it, else <doctrine>.2, .3, ...
func (c *Campaign) newFactionKey(doctrine string) string {
	base := LoadFactionData().FactionKeyFor(doctrine) // a view's legacy key ("mirror")
	if c.FactionOf(base) == nil {
		return base
	}
	for n := 2; ; n++ {
		if k := fmt.Sprintf("%s.%d", base, n); c.FactionOf(k) == nil {
			return k
		}
	}
}

// AddFaction adds a faction instance of doctrine from rival box slot in
// group (0: a group of its own) and returns it. A second faction of a
// doctrine gets a name and a shifted badge.
func (c *Campaign) AddFaction(doctrine string, slot, group int) *Faction {
	if c.FactionTable == nil {
		c.FactionTable = map[string]*Faction{}
	}
	if group == 0 {
		group = c.nextGroup()
	}
	key := c.newFactionKey(doctrine)
	f := &Faction{Key: key, Doctrine: doctrine, Slot: slot, Group: group}
	if n := c.factionsOfDoctrine(doctrine); n > 0 {
		f.Name, f.Badge = c.secondFactionLook(doctrine, n)
		f.Ordinal = n + 1
	}
	c.FactionTable[key] = f
	return f
}

// addFactionRow adds a row for an existing key (the player, the expedition,
// a save's faction) unless it has one.
func (c *Campaign) addFactionRow(key, doctrine string, slot, group int) {
	if key == "" || c.FactionOf(key) != nil {
		return
	}
	if c.FactionTable == nil {
		c.FactionTable = map[string]*Faction{}
	}
	if group == 0 {
		group = c.nextGroup()
	}
	c.FactionTable[key] = &Faction{Key: key, Doctrine: doctrine, Slot: slot, Group: group}
}

// factionsOfDoctrine counts the rival rows running doctrine (the player's
// own and the observer expedition never count: a hive player's one rival
// hive is not "the second").
func (c *Campaign) factionsOfDoctrine(doctrine string) int {
	n := 0
	for _, f := range c.FactionTable {
		if f.Doctrine != doctrine || f.Key == ExpeditionFaction || c.Player != nil && f.Key == c.Player.Faction {
			continue
		}
		n++
	}
	return n
}

// Factions is the table's rows by key.
func (c *Campaign) Factions() []*Faction {
	out := make([]*Faction, 0, len(c.FactionTable))
	for _, f := range c.FactionTable {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// EnsureFactionTable gives every faction the campaign holds a row: the
// player (group 1), the observer expedition (group 1: the player's seat),
// then every presence's and fleet's faction in key order, each in a group
// of its own (a save from before the table, a count-seeded campaign).
func (c *Campaign) EnsureFactionTable() {
	if c.Player != nil {
		c.addFactionRow(c.Player.Faction, c.Player.Doctrine, 0, PlayerGroup)
	}
	keys := map[string]string{}
	for _, loc := range c.Locations {
		for i := range loc.Factions {
			keys[loc.Factions[i].Faction] = loc.Factions[i].Doctrine
		}
	}
	for i := range c.Fleets {
		keys[c.Fleets[i].Presence.Faction] = c.Fleets[i].Presence.Doctrine
	}
	if _, ok := keys[ExpeditionFaction]; ok {
		c.addFactionRow(ExpeditionFaction, DefaultPlayerDoctrine, 0, PlayerGroup)
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		c.addFactionRow(k, keys[k], 0, 0)
	}
}

// secondFactionLook is the name and badge of the (n+1)th faction of
// doctrine: the doctrine's instance_names in turn (else its name and a
// number), its badge lightened and darkened by turns.
func (c *Campaign) secondFactionLook(doctrine string, n int) (string, [3]int) {
	d := LoadFactionData().Doctrine(doctrine)
	if d == nil {
		return fmt.Sprintf("%s %d", doctrine, n+1), [3]int{}
	}
	name := fmt.Sprintf("%s %d", d.Name, n+1) // DRAFT
	if len(d.InstanceNames) > 0 {
		name = d.InstanceNames[(n-1)%len(d.InstanceNames)]
	}
	var b [3]int
	k := 0.35 * float64((n+1)/2)
	for i, v := range d.Badge {
		f := float64(v)
		if n%2 == 1 {
			f += (255 - f) * k // lighter
		} else {
			f *= 1 - k // darker
		}
		b[i] = min(max(int(f), 0), 255)
	}
	return name, b
}

// HeldBySide reports whether loc holds a presence of faction's side (itself
// or a faction of its group, the player's presences included): a faction
// never spreads onto its own side's world (rival_selection_plan.md:
// friendly factions never share a world).
func (c *Campaign) HeldBySide(loc *Location, faction string) bool {
	for i := range loc.Factions {
		if c.SameGroup(loc.Factions[i].Faction, faction) {
			return true
		}
	}
	return false
}

// FactionSuffix is the word a faction's morph-named labels take to tell it
// from another of its doctrine (" 2"); "" for the first or only one.
func (c *Campaign) FactionSuffix(key string) string {
	if f := c.FactionOf(key); f != nil && f.Ordinal > 1 {
		return fmt.Sprintf(" %d", f.Ordinal)
	}
	return ""
}

// FactionBadge is a faction's Star Map colour: its own, else its
// doctrine's.
func (c *Campaign) FactionBadge(key string, d *Doctrine) [3]uint8 {
	if f := c.FactionOf(key); f != nil && f.Badge != [3]int{} {
		return [3]uint8{uint8(f.Badge[0]), uint8(f.Badge[1]), uint8(f.Badge[2])}
	}
	if d == nil {
		return [3]uint8{200, 200, 200}
	}
	return d.Badge
}
