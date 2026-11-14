package campaign

import (
	"math"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// Campaign is the persistent root of a landing_party game: the ship hub, the
// space overworld (Locations), and campaign-wide progress. Only one location's
// level is live at a time; every other location is frozen on disk.
// CampaignVersion is the current campaign-save schema version, stamped at
// generation. Version 2 added the generated history and the star date; older
// campaigns are refused on load rather than migrated (see game.LoadCampaign).
// Version 3 added enemy faction presences; a version 2 save migrates with none
// (MigrateCampaign). Version 4 moved the player's known techs onto the
// player's own presence (Player); a version 3 save migrates them there.
const CampaignVersion = 4

type Campaign struct {
	// Version is the save-schema version (see CampaignVersion).
	Version           int                  `json:"version,omitempty"`
	Name              string               `json:"name"`
	Seed              int64                `json:"seed"`
	CurrentLocationID string               `json:"current_location_id"`
	Locations         map[string]*Location `json:"locations"`
	Ship              *ShipState           `json:"ship"`
	// StarDate is the single campaign clock, in days since the campaign began.
	// It counts world ticks (TickStarDate, TicksPerDay a day) wherever the
	// player is, stops when the world clock stops, and jumps forward by each
	// jump's day cost (AdvanceDays). Levels keep their own Day/Hour only for
	// the day-night cycle.
	StarDate history.StarDate `json:"star_date"`
	// StarTick counts world ticks into the current star-date day.
	StarTick int `json:"star_tick,omitempty"`
	// Quests holds per-quest progress (status).
	Quests map[string]*QuestProgress `json:"quests,omitempty"`
	// QuestDefs are the campaign's generated quest definitions, persisted here
	// (the procedural overworld is not re-derivable from a static file).
	// Bound at session start via BindQuests.
	QuestDefs []Quest `json:"quest_defs,omitempty"`
	// GenSeq counts dynamic-generation events; combined with Seed it makes
	// every expansion deterministic and reproducible across save/load.
	GenSeq int `json:"gen_seq,omitempty"`
	// TravelSeq counts jumps; it advances the per-travel quest roll every
	// jump (even on a miss) so the 1d6 chance actually varies, while staying
	// deterministic and persisted.
	TravelSeq int `json:"travel_seq,omitempty"`
	// ScanSeq counts ship-scanner sweeps; combined with Seed it makes each
	// scan's roll/placement deterministic and reproducible across save/load.
	ScanSeq int `json:"scan_seq,omitempty"`
	// Won/Lost are terminal campaign states (reached Home / total wipe).
	Won  bool `json:"won,omitempty"`
	Lost bool `json:"lost,omitempty"`
	// KilledTargets is the set of quest IDs whose tagged target entity has
	// died (named bosses / bounty creatures). Persisted so completion
	// survives freeze/thaw and save/load.
	KilledTargets map[string]bool `json:"killed_targets,omitempty"`
	// Player is the player's settlement as a faction presence (faction and
	// doctrine "colony" by default, phase C0): its KnownTechs are the techs
	// the colony has researched, kept here (not on a Settlement) so they
	// persist across planets and save/load, where every faction keeps its
	// own (plan, "Two homes for known techs"). It lives on no location and
	// is never stepped by StepFactions. nil on a campaign built by hand;
	// UnlockTech creates it.
	Player *FactionPresence `json:"player,omitempty"`
	// LegacyKnownTechs is a version 3 save's known techs, read only for
	// MigrateCampaign, which moves them onto Player.
	LegacyKnownTechs []string `json:"known_techs,omitempty"`

	// KnownEntities is the set of blueprint IDs the player has hovered at
	// least once. Populated regardless of whether the Encyclopedia is
	// unlocked, so the bestiary is already populated the moment the research
	// finishes. Persisted across save/load.
	KnownEntities []string `json:"known_entities,omitempty"`

	// TravelEdges counts how many times the ship has jumped directly between
	// each pair of locations, keyed by a canonical unordered pair. The Star Map
	// draws these as trade-route lines that darken with use.
	TravelEdges map[string]int `json:"travel_edges,omitempty"`

	// History is the campaign's generated past (civs, figures, site histories)
	// and, later, the record of play. See internal/history.
	History *history.History `json:"history,omitempty"`
	// DatapadsRead are the datapads the crew has picked up, in the order
	// read, with what each said: the Legends tab lists them for re-reading
	// (datapads.go).
	DatapadsRead []DatapadRead `json:"datapads_read,omitempty"`

	// Tutorial is the guided first landing's progress; nil when the expedition
	// started without it (and on every older save).
	Tutorial *TutorialProgress `json:"tutorial,omitempty"`

	// FactionCount is the New Expedition "Rival factions" choice the campaign
	// was created with; it scales spread pressure (see faction.go).
	FactionCount int `json:"faction_count,omitempty"`
	// Observer marks an observer run (observer.go): started with observer
	// mode on, no player ship, the expedition a fleet-carried presence.
	// Saved; the whole run is one.
	Observer bool `json:"observer,omitempty"`
	// FactionDay is the last star-date day StepFactions has stepped.
	FactionDay history.StarDate `json:"faction_day,omitempty"`
	// FactionSeq numbers presences (FactionPresence.ID).
	FactionSeq int `json:"faction_seq,omitempty"`
	// Fleets are things in flight between locations (eggs a launcher fired)
	// and faction ships, flying or landed; numbered by FleetSeq.
	Fleets   []Fleet `json:"fleets,omitempty"`
	FleetSeq int     `json:"fleet_seq,omitempty"`
	// Diplomacy is each faction's stance toward the player where it has
	// changed from its doctrine's (faction_ship.go, Stance).
	Diplomacy map[string]string `json:"diplomacy,omitempty"`

	// FactionTable is the campaign's faction instances by key
	// (faction_table.go): additive, rebuilt from the presences for a save
	// from before it.
	FactionTable map[string]*Faction `json:"faction_table,omitempty"`
	// RivalPicks are the rival boxes the run was started with (nil: the
	// Rival factions count alone), for the sidecar and debugging.
	RivalPicks []RivalPick `json:"rival_picks,omitempty"`
	// Encounter is a faction ship met in space awaiting the player's
	// answer, or just answered (phase D; faction_intercept.go).
	Encounter *Encounter `json:"encounter,omitempty"`
	// EncounterSeq numbers boarding locations.
	EncounterSeq int `json:"encounter_seq,omitempty"`

	questByID map[string]*Quest               `json:"-"`
	questEval map[string]*objective.Evaluator `json:"-"`

	// shipOrders are the factions whose mothership builds a ship this day
	// (phase D; built after the fleet pass, never saved).
	shipOrders []string
}

// Day is the star date as a plain day count, for displays and end screens.
func (c *Campaign) Day() int { return int(c.StarDate) }

// TickStarDate advances the campaign clock by one world tick. The world manager
// calls it once per frame of simulation, whether a level is live or the player
// is on the dashboard with only background planets ticking, and never while the
// world clock is stopped.
func (c *Campaign) TickStarDate() {
	c.StarTick++
	if c.StarTick >= history.TicksPerDay {
		c.StarTick = 0
		c.StarDate++
	}
}

// AdvanceDays moves the star date on by whole days (a jump's travel time).
func (c *Campaign) AdvanceDays(days int) {
	if days > 0 {
		c.StarDate += history.StarDate(days)
	}
}

// JumpDays is the travel time of a jump costing fuel: ceil(fuel / N) days, N
// being generation.json's jump_fuel_per_day. A free jump takes no time.
func JumpDays(fuel int) int {
	if fuel <= 0 {
		return 0
	}
	per := genConfig().JumpFuelPerDay
	if per < 1 {
		per = 1
	}
	return (fuel + per - 1) / per
}

// travelEdgeKey builds the canonical (order-independent) key for a location pair.
func travelEdgeKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// RecordTravel increments the traversal count for the fromID↔toID route.
func (c *Campaign) RecordTravel(fromID, toID string) {
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	if c.TravelEdges == nil {
		c.TravelEdges = map[string]int{}
	}
	c.TravelEdges[travelEdgeKey(fromID, toID)]++
}

// TravelRoute is one traversed route between two locations with its use count.
type TravelRoute struct {
	A, B  string
	Count int
}

// TravelRoutes returns every recorded route, for rendering on the Star Map.
func (c *Campaign) TravelRoutes() []TravelRoute {
	out := make([]TravelRoute, 0, len(c.TravelEdges))
	for k, n := range c.TravelEdges {
		if i := strings.IndexByte(k, 0); i >= 0 {
			out = append(out, TravelRoute{A: k[:i], B: k[i+1:], Count: n})
		}
	}
	return out
}

// HomeKind marks the single "Home" destination; travelling there wins the run.
const HomeKind = "home"

// AdriftKind marks the Wreck Site: the empty point in space where the damaged
// life-raft came to rest and every expedition begins. It has no map or
// scenario, never generates a level, and cannot be landed on, so the first act
// of a campaign is choosing a destination. Travelling there again is refused.
const AdriftKind = "adrift"

// AdriftLocationID and StartLocationID are the fixed IDs of the Wreck Site and
// the generated start system.
const (
	AdriftLocationID = "adrift"
	StartLocationID  = "start"
)

// IsAdrift reports whether the ship sits at the Wreck Site, with nothing to
// orbit or land on.
func (c *Campaign) IsAdrift() bool {
	loc := c.CurrentLocation()
	return loc != nil && loc.Kind == AdriftKind
}

// HasSite reports whether a location can hold a level: not Home, not the Wreck Site.
func (l *Location) HasSite() bool {
	return l != nil && l.Kind != HomeKind && l.Kind != AdriftKind
}

// HomeLocation returns the Home destination descriptor, or nil.
func (c *Campaign) HomeLocation() *Location {
	for _, loc := range c.Locations {
		if loc.Kind == HomeKind {
			return loc
		}
	}
	return nil
}

// DefaultPlayerDoctrine is the player's doctrine when nothing chooses one:
// today's colony. A title-screen race choice (phase F) sets
// GenOptions.Doctrine instead.
const DefaultPlayerDoctrine = "colony"

// NewPlayerPresence is the player's presence running doctrine ("" is
// DefaultPlayerDoctrine).
func NewPlayerPresence(doctrine string) *FactionPresence {
	if doctrine == "" {
		doctrine = DefaultPlayerDoctrine
	}
	faction := doctrine
	if doctrine != DefaultPlayerDoctrine {
		// A race (phase F) plays under the player's own faction key, never
		// its doctrine's, so a rival of the same doctrine (rival greys, a
		// rival hive) stays a rival.
		faction = PlayerFaction
	}
	return &FactionPresence{ID: PlayerPresenceID, Faction: faction, Doctrine: doctrine, LastShotDay: NeverFired}
}

// PlayerPresenceID is the player's presence's ID; FactionSeq never makes it.
const PlayerPresenceID = "player"

// KnownTechs is the tech keys the colony has researched (the player
// presence's).
func (c *Campaign) KnownTechs() []string {
	if c.Player == nil {
		return nil
	}
	return c.Player.KnownTechs
}

// HasTech reports whether the campaign has researched the given tech key.
func (c *Campaign) HasTech(key string) bool {
	for _, k := range c.KnownTechs() {
		if k == key {
			return true
		}
	}
	return false
}

// UnlockTech adds key to the player's known techs if not already present.
func (c *Campaign) UnlockTech(key string) {
	if c.HasTech(key) {
		return
	}
	if c.Player == nil {
		c.Player = NewPlayerPresence("")
	}
	c.Player.KnownTechs = append(c.Player.KnownTechs, key)
}

// Ship-scanner research chains. A level is the count of consecutive techs
// researched from the front of each chain.
var (
	scannerPowerTechs   = []string{"ship_scanner_1", "ship_scanner_2", "ship_scanner_3"}
	scanEfficiencyTechs = []string{"scan_efficiency_1", "scan_efficiency_2"}
)

func (c *Campaign) techChainLevel(chain []string) int {
	n := 0
	for _, k := range chain {
		if !c.HasTech(k) {
			break
		}
		n++
	}
	return n
}

// ScannerLevel is the ship scanner's power level (0 = no scanner researched).
func (c *Campaign) ScannerLevel() int { return c.techChainLevel(scannerPowerTechs) }

// ScanFuelCost is the fuel one scan burns at the current efficiency research.
func (c *Campaign) ScanFuelCost() int {
	costs := genConfig().ScanFuelCosts
	if len(costs) == 0 {
		return 0
	}
	lvl := c.techChainLevel(scanEfficiencyTechs)
	if lvl >= len(costs) {
		lvl = len(costs) - 1
	}
	return costs[lvl]
}

// KnownTechSet returns KnownTechs as a set for O(1) lookup.
func (c *Campaign) KnownTechSet() map[string]bool {
	set := make(map[string]bool, len(c.KnownTechs()))
	for _, k := range c.KnownTechs() {
		set[k] = true
	}
	return set
}

// IsKnownEntity reports whether blueprint has been registered in the
// Encyclopedia (i.e. the player has hovered an entity of this type at least
// once).
func (c *Campaign) IsKnownEntity(blueprint string) bool {
	for _, b := range c.KnownEntities {
		if b == blueprint {
			return true
		}
	}
	return false
}

// AddKnownEntity registers blueprint in the Encyclopedia. Returns true if the
// blueprint was newly added; false if it was already known (or empty). The
// boolean lets the hover hot path emit a one-shot "newly catalogued" message
// without keeping a separate dedup set.
func (c *Campaign) AddKnownEntity(blueprint string) bool {
	if blueprint == "" || c.IsKnownEntity(blueprint) {
		return false
	}
	c.KnownEntities = append(c.KnownEntities, blueprint)
	return true
}

// KnownEntitySet returns KnownEntities as a set for O(1) lookup.
func (c *Campaign) KnownEntitySet() map[string]bool {
	set := make(map[string]bool, len(c.KnownEntities))
	for _, b := range c.KnownEntities {
		set[b] = true
	}
	return set
}

// MarkTargetKilled records that the tagged target for questID has died.
func (c *Campaign) MarkTargetKilled(questID string) {
	if questID == "" {
		return
	}
	if c.KilledTargets == nil {
		c.KilledTargets = map[string]bool{}
	}
	c.KilledTargets[questID] = true
}

// CurrentLocation returns the descriptor for the location the landing party is
// currently on, or nil if none is set.
func (c *Campaign) CurrentLocation() *Location {
	if c == nil {
		return nil
	}
	return c.Locations[c.CurrentLocationID]
}

// FuelCost returns the fuel needed to travel from the ship's current location
// to toID: zero if it is the current location, otherwise the rounded star-map
// distance between them.
func (c *Campaign) FuelCost(toID string) int {
	to := c.Locations[toID]
	if to == nil || toID == c.CurrentLocationID {
		return 0
	}
	from := c.Locations[c.CurrentLocationID]
	if from == nil {
		return 0
	}
	return int(math.Round(math.Hypot(to.X-from.X, to.Y-from.Y)))
}

// DiscoveredLocations returns all locations the player can currently see on the
// star map.
func (c *Campaign) DiscoveredLocations() []*Location {
	var out []*Location
	for _, loc := range c.Locations {
		if loc.Discovered {
			out = append(out, loc)
		}
	}
	return out
}
