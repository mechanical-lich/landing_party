package campaign

import (
	"encoding/json"
	"math"
)

// The player's settlement reads from a doctrine too (phase C0,
// docs/brainstorm/enemy_factions_plan.md, "Playable races"): the build menu,
// the research tree, the printer units, the crafting recipes and the starting
// crew are the doctrine's settlement block. The "colony" doctrine reproduces
// the game as it was before doctrines; a playable race is another doctrine
// with the same block, chosen by GenOptions.Doctrine.

// DoctrineSettlement is what a settlement run by the doctrine can do. The
// lists name entries of the shared content files, so one file serves every
// race: buildables are data/build.json keys, research data/research.json
// keys, units and recipes data/crafting_recipes.json keys.
type DoctrineSettlement struct {
	// Crew is the starting crew's composition (SeedShipCrew).
	Crew []DoctrineCrew `json:"crew"`
	// Needs is whether the crew eats and sleeps (a race that does not skips
	// the needs system). Declared for the race table; the crew blueprints
	// carry the component, and nothing reads the flag until phase F.
	Needs bool `json:"needs"`
	// BuildMenu is the build menu's categories in order. Each lists the
	// build.json entries of its category that are in Buildables (by order,
	// then name); "orders" holds the cursor modes, bound in code.
	BuildMenu  []DoctrineMenuCategory `json:"build_menu"`
	Buildables []string               `json:"buildables"`
	// Research is the tree the settlement can research.
	Research []string `json:"research"`
	// Units are the recipes that make a unit (a printed colonist, a robot);
	// Recipes the rest. Together they are what the crafting modals offer.
	Units   []string `json:"units"`
	Recipes []string `json:"recipes"`
	// Camp is how a rival's camp is laid on its level (phase D; nil: the
	// bunker room every settlement doctrine had in C2).
	Camp *DoctrineCamp `json:"camp,omitempty"`
	// Director is how the faction director runs this settlement (phase C1;
	// internal/director): observer mode for the player's, later the AI's.
	Director *DoctrineDirector `json:"director,omitempty"`
	// Diet is the material tags its workers eat besides food, one item of
	// a stack at a time (the hive: biomass). hive_plan.md.
	Diet []string `json:"diet,omitempty"`
	// Larvae is the larva timer (the hive's grey eggs); nil for none.
	Larvae *DoctrineLarvae `json:"larvae,omitempty"`
	// KillDrops are items a kill by one of its workers drops into the
	// killer's bag, on top of the victim's own drops (corpses as biomass).
	KillDrops map[string]int `json:"kill_drops,omitempty"`
	// Bloom is a choppable that grows on the settlement's floor (the
	// hive's resin bloom: open question 7's farm), nil for none.
	Bloom *DoctrineBloom `json:"bloom,omitempty"`
}

// DoctrineBloom: every EveryTicks a Blueprint grows on an open tile of the
// settlement's conversion floor within its radius, while fewer than one per
// PerTiles such tiles stand.
type DoctrineBloom struct {
	Blueprint  string `json:"blueprint"`
	EveryTicks int    `json:"every_ticks"`
	PerTiles   int    `json:"per_tiles"`
}

// DoctrineLarvae is a settlement's larva timer: a Blueprint larva appears
// beside each of its Stations every EveryTicks, while the settlement holds
// fewer than Cap + PerCluster per standing Stations[1:] structure (the egg
// clusters).
type DoctrineLarvae struct {
	Blueprint  string   `json:"blueprint"`
	EveryTicks int      `json:"every_ticks"`
	Cap        int      `json:"cap"`
	PerCluster int      `json:"per_cluster"`
	Stations   []string `json:"stations"`
}

// DoctrineCamp is a stamped camp: a structure script and its params at the
// presence's centre (the greyish: their landed saucer), the settlement's
// storage claimed inside it, and Station placed by its centre.
type DoctrineCamp struct {
	Stamp   string         `json:"stamp"`
	Params  map[string]any `json:"params,omitempty"`
	W       int            `json:"w"`
	H       int            `json:"h"`
	Station string         `json:"station,omitempty"`
	// Staged: the camp is the presence's stage stamp (phase E, the autonz'
	// node then hub), stamped again over the last when the stage rises;
	// Stamp is then unused.
	Staged bool `json:"staged,omitempty"`
	// Storage is the blueprint a staged camp places when its stamp holds no
	// storage (the hive's biomass pool; default a storage locker).
	Storage string `json:"storage,omitempty"`
}

// DoctrineDirector is a settlement director's table. Priorities are walked
// in order every EveryTicks ticks: an unmet entry orders its next step. The
// ones that spend materials (build, research, craft) share MaxOpen open
// orders, so the higher ones get the stock first; mining and hauling run
// beside them.
type DoctrineDirector struct {
	EveryTicks int `json:"every_ticks"`
	MaxOpen    int `json:"max_open"`
	// Radius is how far from the settlement centre it builds, and counts
	// structures as its own.
	Radius int `json:"radius"`
	// Reach is how far it looks for deposits to mine and loose things to haul.
	Reach int `json:"reach"`
	// MaxMines is the most mining orders it keeps open per resource;
	// MaxHauls the most haul orders.
	MaxMines int `json:"max_mines"`
	MaxHauls int `json:"max_hauls"`
	// StuckTicks: an order of its own that has made no progress for this
	// long is cancelled (the Cancel Task order) and its site avoided.
	StuckTicks int `json:"stuck_ticks"`
	// DigDepth is how many levels below the centre it digs for a resource
	// that no reachable face yields (0: it never digs); DigReach how far from
	// the centre, across, it looks for that ore below and tunnels to it.
	DigDepth int `json:"dig_depth,omitempty"`
	DigReach int `json:"dig_reach,omitempty"`
	// MaxHunts is the most hunting orders it keeps open (phase D; default
	// 2); HuntTicks how long one may run before it is called off (default
	// 750); HuntReach how far from the centre it hunts (default twice
	// Reach: game wanders).
	MaxHunts   int                `json:"max_hunts,omitempty"`
	HuntTicks  int                `json:"hunt_ticks,omitempty"`
	HuntReach  int                `json:"hunt_reach,omitempty"`
	Priorities []DirectorPriority `json:"priorities"`
}

// DirectorRaid is a raid rung (phase D): a hostile faction settlement sends
// a party against the player's nearest colonist (or a storage container of
// the player's that can be hit) when it has enough fighters, then calls it
// off. Neutral settlements, and the player's own, never raid.
type DirectorRaid struct {
	// Fighters is the armed workers wanted before a raid at aggression 5;
	// the threshold scales by 5 / aggression (an aggression 8 faction goes
	// with fewer, a timid one waits for more), at least 1.
	Fighters int `json:"fighters"`
	// Party is the most attack orders a raid gives (at most the fighters).
	Party int `json:"party"`
	// CooldownTicks between raids at aggression 5, scaled the same way;
	// MaxTicks a raid may run before the party is called home.
	CooldownTicks int `json:"cooldown_ticks"`
	MaxTicks      int `json:"max_ticks"`
	// Reach is how far from the settlement centre it looks for a target.
	Reach int `json:"reach"`
	// Home is how many workers a raid leaves at the camp (at least 1).
	Home int `json:"home,omitempty"`
	// Roles: only workers whose crew role is one of these fight (the
	// greyish send their soldiers); none: any worker.
	Roles []string `json:"roles,omitempty"`
	// Unarmed: every worker counts as a fighter (a race that fights with
	// what it is born with); otherwise only workers holding a weapon, and
	// the rung equips stored weapons first.
	Unarmed bool `json:"unarmed,omitempty"`
	// Scouted: only once a scout has seen the player's colony (the hive's
	// contact; hive_plan.md).
	Scouted bool `json:"scouted,omitempty"`
	// Leader: once the settlement can raise this type (the autonz' warden),
	// a party forms only around one of them, which leads it.
	Leader string `json:"leader,omitempty"`
}

// DirectorGuard: workers of Roles hold posts Post tiles out from the
// centre; a hostile within Radius of the centre is set upon by every one,
// and they go back to their posts once it is dead or gone.
type DirectorGuard struct {
	Roles  []string `json:"roles"`
	Radius int      `json:"radius"`
	Post   int      `json:"post"`
	// Posts: one guard holds beside each standing structure of this
	// blueprint instead of a ring (the autonz' sentinels at defence pylons).
	Posts string `json:"posts,omitempty"`
}

// DirectorDeploy: while fewer of the As structure stand than the stage
// holds, a unit of Type is printed if none is idle, then walked to a wired
// tile at least Edge from the centre, toward the nearest site its signal
// could reach, and deployed there (the autonz' relay drone; autonz_plan.md).
type DirectorDeploy struct {
	Type string `json:"type"`
	As   string `json:"as"`
	Edge int    `json:"edge"`
}

// DirectorLaunch: on a day the launcher would fire (Site.LaunchReady),
// the shot's bodies (the economy types first) walk to stand within Walk
// tiles of the spire, where the day's live rule takes them.
type DirectorLaunch struct {
	Walk int `json:"walk"`
	// At names the structure the bodies walk to (the autonz' signal_core;
	// default the doctrine's launcher); Roles, when set, are the only roles
	// sent, in order of preference (wire placers, then relay drones).
	At    string   `json:"at,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

// DirectorPatrol: idle workers of Roles are sent to a waypoint on a ring
// Radius from the centre every EveryTicks.
type DirectorPatrol struct {
	Roles      []string `json:"roles"`
	Radius     int      `json:"radius"`
	EveryTicks int      `json:"every_ticks"`
}

// DirectorExcavate: workers of Roles dig the rock nearest the centre within
// Radius when the convert rung found nothing to lay its floor on.
type DirectorExcavate struct {
	Roles  []string `json:"roles"`
	Radius int      `json:"radius"`
}

// DirectorPriority is one rung of the needs ladder. Exactly one of Build,
// Research, Craft, Mine, Haul, Dig, Convert, Hunt, Raid and Grow says what it orders:
//   - build: a build.json key, wanted Min + PerColonist * colonists times
//     within Radius (built, or ordered);
//   - research: the next tech of the settlement's tree, one at a time, at a
//     research building;
//   - craft: a recipe, at its station, until Min + PerColonist * colonists
//     of its output are stored or carried;
//   - mine: resource -> stock wanted in storage; mines deposits and chops
//     what drops it until then;
//   - haul: "food" (or a blueprint): loose items of it brought into storage;
//   - dig: for each resource a mine rung wanted and found no face for, the
//     next order toward ore below (DigDepth, DigReach). Where it sits in
//     the ladder is what it may spend on stairs: what the rungs above it
//     left, and the rest of the shaft is held back from the rungs below;
//   - hunt: "food": attack orders on creatures that drop food (critters),
//     nearest first, until Min + PerColonist * colonists food items are
//     stored or carried (the haul rung brings the drops home), and the
//     doctrine's food recipe crafted (its input chopped or mined first),
//     once someone's hunger reaches Hunger;
//   - raid: see DirectorRaid;
//   - grow: see Grow;
//   - convert: a floor tile; ConvertAction orders on the reachable ground
//     nearest the centre until Min + PerColonist * colonists tiles within
//     Radius are it (the conversion economy; no doctrine uses it yet).
type DirectorPriority struct {
	ID       string         `json:"id"`
	Build    string         `json:"build,omitempty"`
	Research bool           `json:"research,omitempty"`
	Craft    string         `json:"craft,omitempty"`
	Mine     map[string]int `json:"mine,omitempty"`
	Haul     string         `json:"haul,omitempty"`
	Dig      bool           `json:"dig,omitempty"`
	Convert  string         `json:"convert,omitempty"`
	Hunt     string         `json:"hunt,omitempty"`
	// Grow: bio-print the doctrine's units at their station until the crew
	// reaches the site's crew cap (a ship's tier), the type most under its
	// crew share first (phase D, the greyish). `true` in the file, or (a
	// typed doctrine, the hive) an object naming a type to keep at Min +
	// Share x bodies (hive_plan.md, "The director table").
	Grow *DirectorGrow `json:"grow,omitempty"`
	// Hunger: a hunt rung acts once the hungriest colonist's hunger reaches
	// this (0: always), so the opening's builds come first.
	Hunger int           `json:"hunger,omitempty"`
	Raid   *DirectorRaid `json:"raid,omitempty"`
	// The hive's rungs (hive_plan.md, "The director table"): Guard holds
	// its guards at posts round the heart and sets them on a hostile inside
	// the radius; Launch walks a shot's bodies to the spire on a day the
	// launcher would fire; Patrol sends scouts round a ring (a scout seeing
	// the player's colony records contact); Excavate digs rock near the
	// heart when resin has no floor to spread on.
	Guard    *DirectorGuard    `json:"guard,omitempty"`
	Launch   *DirectorLaunch   `json:"launch,omitempty"`
	Patrol   *DirectorPatrol   `json:"patrol,omitempty"`
	Excavate *DirectorExcavate `json:"excavate,omitempty"`
	Deploy   *DirectorDeploy   `json:"deploy,omitempty"`
	// ConvertOrigins: the convert rung works outward from the centre and
	// from each standing structure of these blueprints (the autonz'
	// converter nodes).
	ConvertOrigins []string `json:"convert_origins,omitempty"`
	// AtCap: the convert rung keeps laying while the crew is at its crew
	// cap, whatever its want (the autonz, whose cap is its conversion).
	AtCap       bool    `json:"at_cap,omitempty"`
	Min         int     `json:"min,omitempty"`
	PerColonist float64 `json:"per_colonist,omitempty"`
	// StageMin: the rung runs only once the presence has reached this stage
	// (the hive's chamber at the nest, its spire at the launcher stage).
	StageMin int `json:"stage_min,omitempty"`
}

// Want is how many the entry wants for a settlement of colonists.
func (p DirectorPriority) Want(colonists int) int {
	return p.Min + int(math.Ceil(p.PerColonist*float64(colonists)))
}

// DirectorGrow is a grow rung. Type "" grows the crew to its cap (the
// greyish: the crew share; a typed doctrine: the type most under the stage's
// mix among unlocked types); a Type keeps that type's bodies (adults, young
// and eggs) at Min + Share x bodies.
type DirectorGrow struct {
	Type  string  `json:"type,omitempty"`
	Min   int     `json:"min,omitempty"`
	Share float64 `json:"share,omitempty"`
	// Reserve is the stock per body an egg must leave behind (a hungry hive
	// feeds before it breeds); in the economy's resource.
	Reserve float64 `json:"reserve,omitempty"`
}

// UnmarshalJSON reads `true` (grow to the cap), `false` (no rung) or an
// object.
func (g *DirectorGrow) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true":
		*g = DirectorGrow{}
		return nil
	case "false", "null":
		return nil
	}
	type plain DirectorGrow
	return json.Unmarshal(b, (*plain)(g))
}

// DoctrineCrew is one kind of starting crew member and its share of the crew.
type DoctrineCrew struct {
	Blueprint string  `json:"blueprint"`
	Share     float64 `json:"share"`
	// Role is what the type is for (the greyish: miner, hauler, soldier,
	// scientist); the director's rungs read it (a raid's fighters).
	Role string `json:"role,omitempty"`
	// Type is the doctrine unit type the crew member is given (the hive's
	// drone, scout); factory.ApplyType.
	Type string `json:"type,omitempty"`
}

// DoctrineMenuCategory is one build menu category.
type DoctrineMenuCategory struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// PlayerDoctrine is the doctrine the player's settlement runs: the campaign's
// player presence's, else DefaultPlayerDoctrine (no campaign, or an old save
// before migration). nil only when the doctrine file lacks it.
func PlayerDoctrine(c *Campaign) *Doctrine {
	return LoadFactionData().Doctrine(PlayerDoctrineKey(c))
}

// PlayerDoctrineKey is PlayerDoctrine's key.
func PlayerDoctrineKey(c *Campaign) string {
	if c != nil && c.Player != nil && c.Player.Doctrine != "" {
		return c.Player.Doctrine
	}
	return DefaultPlayerDoctrine
}

// CampaignMode is the campaign shape the doctrine's mobility implies for a
// player running it (plan, "Playable races"): "ship" (hold, roster, fuel,
// travel) or "launcher" (no ship; phase F). Declared; nothing reads it yet.
func (d *Doctrine) CampaignMode() string {
	if d.Mobility.Kind == "launcher" {
		return "launcher"
	}
	return "ship"
}

// CanBuild reports whether the doctrine's settlement may build a build.json
// entry.
func (s *DoctrineSettlement) CanBuild(key string) bool {
	return s != nil && contains(s.Buildables, key)
}

// CanResearch reports whether research.json entry key is in the tree.
func (s *DoctrineSettlement) CanResearch(key string) bool {
	return s != nil && contains(s.Research, key)
}

// CanCraft reports whether crafting recipe key is offered (a unit or a
// recipe).
func (s *DoctrineSettlement) CanCraft(key string) bool {
	return s != nil && (contains(s.Units, key) || contains(s.Recipes, key))
}

// CrewBlueprints is n crew members' blueprints: n split by share, the
// remainder to the first entry, in Crew order.
func (s *DoctrineSettlement) CrewBlueprints(n int) []string {
	var out []string
	for _, c := range s.CrewEntries(n) {
		out = append(out, c.Blueprint)
	}
	return out
}

// CrewEntries is CrewBlueprints with each member's crew entry (its type and
// role).
func (s *DoctrineSettlement) CrewEntries(n int) []DoctrineCrew {
	if s == nil || len(s.Crew) == 0 || n <= 0 {
		return nil
	}
	total := 0.0
	for _, c := range s.Crew {
		total += c.Share
	}
	// Largest remainder: whole shares first, then the rest to the largest
	// fractions (ties in list order), so a small crew still mixes its types.
	counts := make([]int, len(s.Crew))
	fracs := make([]float64, len(s.Crew))
	used := 0
	for i, c := range s.Crew {
		if total > 0 {
			v := float64(n) * c.Share / total
			counts[i] = int(v)
			fracs[i] = v - float64(counts[i])
		}
		used += counts[i]
	}
	for ; used < n; used++ {
		best := 0
		for i := range fracs {
			if fracs[i] > fracs[best] {
				best = i
			}
		}
		counts[best]++
		fracs[best] = -1
	}
	out := make([]DoctrineCrew, 0, n)
	for i, c := range s.Crew {
		for j := 0; j < counts[i]; j++ {
			out = append(out, c)
		}
	}
	return out
}

func contains(list []string, key string) bool {
	for _, k := range list {
		if k == key {
			return true
		}
	}
	return false
}

// WithMix is s with its crew shares replaced by mix (blueprint to share)
// where mix names them: a stage's composition (phase E). s itself when mix
// is empty.
func (s *DoctrineSettlement) WithMix(mix map[string]float64) *DoctrineSettlement {
	if s == nil || len(mix) == 0 {
		return s
	}
	cp := *s
	cp.Crew = make([]DoctrineCrew, len(s.Crew))
	for i, c := range s.Crew {
		if v, ok := mix[c.Blueprint]; ok {
			c.Share = v
		} else if v, ok := mix[c.Type]; ok && c.Type != "" {
			c.Share = v
		}
		cp.Crew[i] = c
	}
	return &cp
}

// StageSettlement is d's settlement block at stage i: its crew by the
// stage's mix.
func (d *Doctrine) StageSettlement(i int) *DoctrineSettlement {
	if d == nil || d.Settlement == nil {
		return nil
	}
	if i < 0 || i >= len(d.Stages) {
		return d.Settlement
	}
	return d.Settlement.WithMix(d.Stages[i].Mix)
}

// StageSettlementAt is StageSettlement for a presence on loc knowing techs:
// a typed doctrine's crew by its type mix (locked types' shares given to
// their fallbacks, so an autonz node with nothing unlocked keeps Phase E's
// 0.75 / 0.25).
func (d *Doctrine) StageSettlementAt(i int, loc *Location, techs []string) *DoctrineSettlement {
	if d == nil || d.Settlement == nil || !d.HasTypes() || i < 0 || i >= len(d.Stages) {
		return d.StageSettlement(i)
	}
	return d.Settlement.WithMix(d.TypeMix(i, loc, techs))
}

// RoleOf is a crew blueprint's role ("" when it has none or is not crew).
func (s *DoctrineSettlement) RoleOf(blueprint string) string {
	if s == nil {
		return ""
	}
	for _, c := range s.Crew {
		if c.Blueprint == blueprint {
			return c.Role
		}
	}
	return ""
}

// ShareOf is a crew blueprint's share of the crew (0 when not crew).
func (s *DoctrineSettlement) ShareOf(blueprint string) float64 {
	if s == nil {
		return 0
	}
	total, mine := 0.0, 0.0
	for _, c := range s.Crew {
		total += c.Share
		if c.Blueprint == blueprint {
			mine += c.Share
		}
	}
	if total <= 0 {
		return 0
	}
	return mine / total
}
