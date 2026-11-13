package campaign

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// Enemy factions, campaign tier (docs/brainstorm/enemy_factions_plan.md;
// docs/developer/factions.md). A faction lives on a location as a small
// FactionPresence record advanced once per star-date day by StepFactions;
// the game builds it onto its site's level and reconciles it back
// (internal/game/faction_level.go). Presences sit on top of the site's generated
// history: they never change it, they only read the scenario it picked.

// FactionsPath is the doctrine file. Overridable in tests.
var FactionsPath = "data/factions.json"

// FactionPresence is one faction's hold on one place. Planet-carried factions
// keep it on Location.Factions; a later fleet carrier embeds the same record.
//
// KnownTechs live here for every faction, the player's included: the
// player's settlement owns a presence, Campaign.Player (plan, "Two homes for
// known techs"; phase C0).
type FactionPresence struct {
	// ID is unique within the campaign ("fp3"), for events and later the
	// level's faction settlement.
	ID       string `json:"id"`
	Faction  string `json:"faction"`  // display key, e.g. "hive"
	Doctrine string `json:"doctrine"` // key into data/factions.json
	// Population is the number of bodies. Growth carries the fractional part
	// between days (positive toward the next body, negative toward a loss).
	Population int     `json:"population"`
	Growth     float64 `json:"growth,omitempty"`
	// Units is a typed doctrine's bodies by type (the hive: drones,
	// warriors, ...), summing to Population: Tier 1 bookkeeping the live
	// level is reconciled into (doctrine_types.go). nil for an untyped one.
	Units map[string]int `json:"units,omitempty"`
	// Built is a structure-building settlement's standing structures by
	// blueprint (the autonz' cogitator, foundry, pylons; autonz_plan.md):
	// Tier 1 keeps it at the stage's structures its techs allow, reconcile
	// takes what stands on the level, materialise places it. Bookkeeping:
	// it never blocks growth. nil is unknown (materialise uses the stage's
	// set); an empty map is nothing standing (razed), kept across a save, so
	// the field has no omitempty. Tier 1 rebuilds one missing structure a day.
	Built map[string]int `json:"built"`
	// Gear is a ship settlement's weapons and armour held or worn by its
	// bodies, by blueprint (the greys, phase E4): reconcile counts them,
	// materialise hands them out; nil is unknown (materialise uses the
	// doctrine's gear floor for the units and techs).
	Gear map[string]int `json:"gear,omitempty"`
	// Larvae is a hive's unassigned larvae (grey eggs), not counted in
	// Population; Tier 1 accrues them, the level holds them as hive_larva.
	Larvae int `json:"larvae,omitempty"`
	// Stage indexes the doctrine's stages (0 brood, 1 nest, 2 hive); it only
	// rises: a structure stays built when the numbers fall.
	Stage int `json:"stage"`
	// Stockpile and Conversion are the strip / convert economies' tallies.
	// The hive doctrine works neither in Phase A; the fields are here so the
	// record is the one Tier 2 reconciles into.
	Stockpile  map[string]int `json:"stockpile,omitempty"`
	Conversion float64        `json:"conversion,omitempty"`

	ArrivedDay  history.StarDate `json:"arrived_day"`
	LastStepDay history.StarDate `json:"last_step_day"`
	// LastShotDay is the launcher cooldown's start; NeverFired (-1) until
	// the first shot, which waits only on growth.
	LastShotDay history.StarDate `json:"last_shot_day"`

	KnownTechs  []string `json:"known_techs,omitempty"`
	ResearchPts float64  `json:"research_pts,omitempty"`
	// AssimPoints are the second walk's points (the autonz' assimilation;
	// Doctrine.Assimilation).
	AssimPoints float64 `json:"assim_points,omitempty"`

	// Seed makes a later materialisation of this presence deterministic.
	Seed int64 `json:"seed"`
	// Fuel is a ship-carried presence's fuel, when its doctrine flies by an
	// expedition table: a jump costs its distance, as the player's does.
	Fuel int `json:"fuel,omitempty"`

	// Known is whether the player has seen this presence (orbit, scan). The
	// Star Map draws known presences only; Events are the live-history events
	// recorded while it was unknown, revealed with it.
	Known  bool     `json:"known,omitempty"`
	Events []string `json:"events,omitempty"`

	// Tier 2 (the level). Materialized is set once the presence has been
	// built onto its site's level; LevelStage is the highest stage whose
	// stamp stands there. LauncherLost is set when reconcile finds the
	// launcher stage's launcher destroyed (LauncherLostDay); the hive
	// regrows it after its cooldown (Tier 1) and the next materialise stands
	// it again.
	Materialized    bool             `json:"materialized,omitempty"`
	LevelStage      int              `json:"level_stage,omitempty"`
	LauncherLost    bool             `json:"launcher_lost,omitempty"`
	LauncherLostDay history.StarDate `json:"launcher_lost_day,omitempty"`
	// LauncherStood: reconcile has seen the launcher standing since it was
	// last lost. Only a launcher that stood can be lost: a hive that rises to
	// the launcher stage live has no spire until its director builds one,
	// and that is not a loss (it would block Tier 1's fire for a cooldown).
	LauncherStood bool `json:"launcher_stood,omitempty"`
	// Camped: the presence's camp stood on its level when it was last
	// written (a frozen level keeps it; phase D4 ruins it on departure).
	Camped bool `json:"camped,omitempty"`
	// YieldFrac carries the fraction of a ship's daily digging per resource
	// (phase D; whole units go into Stockpile).
	YieldFrac map[string]float64 `json:"yield_frac,omitempty"`
	// Dormant: founded by a signal (phase E, the autonz) and asleep until
	// WakeDay: it neither grows, works, researches nor spreads, and on its
	// level it is a single sleeper node.
	Dormant bool             `json:"dormant,omitempty"`
	WakeDay history.StarDate `json:"wake_day,omitempty"`
	// DoorLostTick is the level tick a ship settlement's last storage door
	// fell (live only; 0 while a door stands): a day later the ship
	// re-places its cockpit door (greyish_plan.md, "The storage pocket").
	DoorLostTick int `json:"door_lost_tick,omitempty"`
	// BeamStock is a mothership's stock for beaming down to its ships'
	// camps (DoctrineBeam), apart from its hoard (Stockpile).
	BeamStock map[string]int `json:"beam_stock,omitempty"`
	// Beamed is the units a landed ship's camp holds that its mothership
	// beamed down (not yet spent; Fleet.NoteBeamed), left out of its hold
	// target (Fleet.HoldForHome); BeamLastHold its storage after the last
	// beam. Live only: Tier 1 never beams.
	Beamed       int `json:"beamed,omitempty"`
	BeamLastHold int `json:"beam_last_hold,omitempty"`
}

// FactionHusk is what an exhausted presence leaves on its site: the level
// stamps the doctrine's husk (cracked eggs) where it lived, once (Stamped),
// and its remaining creatures die out.
type FactionHusk struct {
	Presence string           `json:"presence"` // the dead presence's ID
	Faction  string           `json:"faction"`
	Doctrine string           `json:"doctrine"`
	Morph    string           `json:"morph,omitempty"` // whose eggs the husk shows
	Day      history.StarDate `json:"day"`
	Seed     int64            `json:"seed"`
	Stamped  bool             `json:"stamped,omitempty"`
}

// Fleet is something travelling between locations on the Star Map. Phase B
// has one kind, FleetProjectile: an egg a launcher fired, carrying the brood
// it lands as (Presence: faction, doctrine, population, the parent's techs).
// It moves from (FromX, FromY) to (ToX, ToY) between LaunchDay and
// ArriveDay (Pos); ships (greyish, the mirror colony) join in phase D.
type Fleet struct {
	ID        string           `json:"id"` // "fl3", unique in the campaign
	Kind      string           `json:"kind"`
	FromID    string           `json:"from_id"`
	ToID      string           `json:"to_id"`
	FromX     float64          `json:"from_x"`
	FromY     float64          `json:"from_y"`
	ToX       float64          `json:"to_x"`
	ToY       float64          `json:"to_y"`
	LaunchDay history.StarDate `json:"launch_day"`
	ArriveDay history.StarDate `json:"arrive_day"`
	Presence  FactionPresence  `json:"presence"`
	// Known: the player has seen it (the hive that fired it was known, or a
	// scanner caught it in flight). The Star Map draws known fleets only.
	Known bool `json:"known,omitempty"`
	// LandedID is the site a ship (Kind "ship") is landed on, "" while it
	// flies; LandedDay when it came down (faction_ship.go).
	LandedID  string           `json:"landed_id,omitempty"`
	LandedDay history.StarDate `json:"landed_day,omitempty"`
	// ShipTier is a ship's tier in its doctrine's ships.tiers (phase D): it
	// caps the crew, and only rises at the faction's mothership.
	ShipTier int `json:"ship_tier,omitempty"`
	// Mothership: the faction's mothership, parked in orbit of its location
	// (FromID = ToID), never landing; Returning: a ship bound for it with
	// its hold (phase D, "The greyish").
	Mothership bool `json:"mothership,omitempty"`
	Returning  bool `json:"returning,omitempty"`
	// BoardedAt is the encounter location of a boarding in progress: the
	// ship is held where it was met until the fight is over (phase D).
	BoardedAt string `json:"boarded_at,omitempty"`
	// LastMove is why a table-flown ship last lifted off: "table" (a site in
	// reach scoring better than staying), "pack_up" (its colony failing),
	// "stay" (its stay_days over, to the table's best site) or "fallback"
	// (its stay over and no site worth it: a free one the fuel reaches, as
	// before the table).
	LastMove string `json:"last_move,omitempty"`
}

// FleetProjectile is a Fleet that is an egg in flight.
const FleetProjectile = "projectile"

// Pos is where f is at star time t (days, fractional): along the line from
// its origin to its target by the share of its flight elapsed, held at the
// target while it waits in orbit.
func (f *Fleet) Pos(t float64) (x, y float64) {
	span := float64(f.ArriveDay - f.LaunchDay)
	k := 1.0
	if span > 0 {
		k = (t - float64(f.LaunchDay)) / span
	}
	k = math.Max(0, math.Min(1, k))
	return f.FromX + (f.ToX-f.FromX)*k, f.FromY + (f.ToY-f.FromY)*k
}

// StarTime is the star date with the day's elapsed ticks as a fraction.
func (c *Campaign) StarTime() float64 {
	return float64(c.StarDate) + float64(c.StarTick)/float64(history.TicksPerDay)
}

// KnownFleets are the fleets the player has seen.
func (c *Campaign) KnownFleets() []*Fleet {
	var out []*Fleet
	for i := range c.Fleets {
		if c.Fleets[i].Known {
			out = append(out, &c.Fleets[i])
		}
	}
	return out
}

// spotFleets marks every fleet within the ship's scanner range known: a
// scanner (ScannerLevel 1+) watches its range from the ship's location.
func (c *Campaign) spotFleets() {
	r := c.scannerRange()
	cur := c.Locations[c.CurrentLocationID]
	if r <= 0 || cur == nil {
		return
	}
	c.revealFleetsWithin(cur.X, cur.Y, r)
}

// revealFleetsWithin marks every fleet within r of (x, y) known now.
func (c *Campaign) revealFleetsWithin(x, y, r float64) {
	t := c.StarTime()
	for i := range c.Fleets {
		if fx, fy := c.Fleets[i].Pos(t); math.Hypot(fx-x, fy-y) <= r {
			c.Fleets[i].Known = true
		}
	}
}

// scannerRange is the range of the ship's scanner, 0 without one.
func (c *Campaign) scannerRange() float64 {
	lvl := c.ScannerLevel()
	tiers := genConfig().ScannerTiers
	if lvl < 1 || len(tiers) == 0 {
		return 0
	}
	return tiers[min(lvl, len(tiers))-1].Range
}

// ---- doctrine data ----

// FactionData is data/factions.json.
type FactionData struct {
	// PressurePerFaction scales spread with the New Expedition faction count:
	// a launcher's cooldown is divided by 1 + PressurePerFaction*(count-1).
	PressurePerFaction float64              `json:"pressure_per_faction"`
	Doctrines          map[string]*Doctrine `json:"doctrines"`
	// Intercept is how ships met in space are resolved (phase D).
	Intercept InterceptRules `json:"intercept"`
}

// Doctrine is one faction's data: how it grows, spreads and stages up.
type Doctrine struct {
	Name     string `json:"name"`     // "Hive"
	Singular string `json:"singular"` // "hive"; player-facing text names the morph (MorphAt)
	Seedable bool   `json:"seedable"` // drawn by New Expedition seeding
	// SeedPopulation is a seeded presence's starting size.
	SeedPopulation int `json:"seed_population"`
	// Morphs are the body plans players meet ("crawler", "flier"), with the
	// names player-facing text uses while the faction's shared identity is
	// hidden (always, in Phase A). MorphByScenario picks a site's morph from
	// its scenario id, else DefaultMorph (see MorphAt).
	Morphs          map[string]*DoctrineMorph `json:"morphs,omitempty"`
	MorphByScenario map[string]string         `json:"morph_by_scenario,omitempty"`
	DefaultMorph    string                    `json:"default_morph,omitempty"`
	// Badge is the Star Map colour.
	Badge    [3]uint8         `json:"badge"`
	Mobility DoctrineMobility `json:"mobility"`
	Economy  string           `json:"economy"` // tag the faction prefers ("biomass")
	Growth   DoctrineGrowth   `json:"growth"`
	Capacity DoctrineCapacity `json:"capacity"`
	// Hazard multiplies growth per scenario id: 0.5 grows at half speed, a
	// negative value shrinks the presence every day until it dies out.
	// Scenarios not listed are 1.
	Hazard map[string]float64 `json:"hazard"`
	Stages []DoctrineStage    `json:"stages"`
	// ResearchPerDay is points per day per stage reached (a brood does none).
	ResearchPerDay float64        `json:"research_per_day"`
	Research       []DoctrineTech `json:"research"`
	// Assimilation is a second abstract walk beside Research (the autonz'
	// unit patterns and bronze tiers; autonz_plan.md), with its own points,
	// AssimilationPerDay per stage, so the spine's days do not move. Its
	// entries have no range or rate.
	AssimilationPerDay []float64      `json:"assimilation_per_day,omitempty"`
	Assimilation       []DoctrineTech `json:"assimilation,omitempty"`
	// SecondWalk and SecondWalkPerDay are the neutral names of the same
	// walk (phase E4): read into Assimilation when it is not set. For a
	// ship the per-day list is by hull tier (greyish_plan.md, "Tier 1: two
	// walks"), for a planet presence by stage.
	SecondWalkPerDay []float64      `json:"second_walk_per_day,omitempty"`
	SecondWalk       []DoctrineTech `json:"second_walk,omitempty"`
	// Aggression seeds the hostility of materialised units (Phase C).
	Aggression int `json:"aggression"`
	// RaidAlertText is the doctrine's own raid alert ({name}, {place});
	// "" for the default lines (RaidAlert).
	RaidAlertText string `json:"raid_alert,omitempty"`
	// Types are the unit types the factory applies at spawn (sprite colour
	// block, stats, role; doctrine_types.go); MorphFallback the type a
	// locked type's share goes to, per morph; TechAliases map an old tech
	// key to the one that replaced it (tier_advanced to the warrior brood).
	Types         map[string]*DoctrineType `json:"types,omitempty"`
	MorphFallback map[string]string        `json:"morph_fallback,omitempty"`
	TechAliases   map[string]string        `json:"tech_aliases,omitempty"`
	// TypeFallback is the type a locked type's share goes to in a doctrine
	// without morphs (the autonz' wire placer); EconomyTypeList the types a
	// shot takes first (else the morphs' fallbacks); Tiers the print tiers
	// (steel, bronze), applied per type at spawn (TierOfType).
	TypeFallback    string         `json:"type_fallback,omitempty"`
	EconomyTypeList []string       `json:"economy_types,omitempty"`
	Tiers           []DoctrineTier `json:"tiers,omitempty"`
	// BodySkills are given to every typed body at every life stage (the
	// hive's radiation_resist: it lives on moons).
	BodySkills []string `json:"body_skills,omitempty"`
	// Biomass is a biomass economy's abstract daily tally (the hive;
	// faction_hive.go).
	Biomass *DoctrineBiomass `json:"biomass,omitempty"`
	// Level is how a presence materialises on its level (Tier 2).
	Level DoctrineLevel `json:"level"`
	// HomeScenarios are the scenarios whose chapter threat is this faction
	// (a hive world): a presence there builds by the chapter's origin, one
	// organism with the scenario's own creatures.
	HomeScenarios []string `json:"home_scenarios,omitempty"`
	// Playable marks a doctrine the player's settlement can run;
	// Settlement is what that settlement can build, research, craft and
	// crew (doctrine_settlement.go). The colony has both; a faction gains
	// them to become a playable race (phase F).
	Playable   bool                `json:"playable,omitempty"`
	Settlement *DoctrineSettlement `json:"settlement,omitempty"`
	// Expedition is how the ship is flown when the game plays itself
	// (observer mode v2; doctrine_expedition.go). The colony has it.
	Expedition *DoctrineExpedition `json:"expedition,omitempty"`
	// ExpeditionFrom names a doctrine whose expedition table this one flies
	// by when it has none of its own (the mirror colony's ship flies the
	// colony's); resolved on load. A fleet doctrine without either keeps its
	// stay_days hop (the greyish).
	ExpeditionFrom string `json:"expedition_from,omitempty"`
	// SettlementFrom names a doctrine whose settlement block this one runs
	// when it has none of its own (the mirror colony plays the colony's
	// game). Resolved on load.
	SettlementFrom string `json:"settlement_from,omitempty"`
	// SeedFrom: seeding draws this doctrine only when the New Expedition
	// faction count is at least this (0: always), so a lone rival is the
	// hive and the second is drawn from a wider bag.
	SeedFrom int `json:"seed_from,omitempty"`
	// Kit is the stockpile a fresh presence founds with (a ship's hold), as
	// the player beams down an opening kit.
	Kit map[string]int `json:"kit,omitempty"`
	// Conversion is the doctrine's conversion economy: the floor its
	// ConvertActions lay, and the radius around a presence's centre its
	// Conversion share covers (the biome patch materialise applies).
	Conversion DoctrineConversion `json:"conversion,omitempty"`
	// Diplomacy is the stance a faction starts at toward the player
	// ("hostile" when empty); see Campaign.Stance.
	Diplomacy string `json:"diplomacy,omitempty"`
	// Ships is a fleet faction's ships (phase D): tiers, marker, the
	// stockpile rhythm and the mothership. nil: one ship, no tiers.
	Ships *DoctrineShips `json:"ships,omitempty"`
	// PlayerName is what player-facing text calls the faction when it is
	// not its Name (phase D; the greyish: "Greys" until known better).
	PlayerName string `json:"player_name,omitempty"`
	// ArrivalEvent is the Legends event a seeded or landed presence records
	// ("" is faction_arrived, the hive's egg).
	ArrivalEvent string `json:"arrival_event,omitempty"`
	// BadgeShape is the Star Map badge's shape: "" a diamond, "square" (the
	// autonz' chip).
	BadgeShape string `json:"badge_shape,omitempty"`
}

// DoctrineConversion is a doctrine's conversion tile and patch radius.
type DoctrineConversion struct {
	Tile   string  `json:"tile,omitempty"`
	Radius int     `json:"radius,omitempty"`
	PerDay float64 `json:"per_day,omitempty"`
	// The convert economy (phase E, the autonz): Cap is the most of the
	// patch a presence converts; Floor the share of the site's capacity it
	// carries with nothing converted (the rest scales with Conversion / Cap,
	// ConvertCap); Tags add to the daily rate by site tag (ore feeds it).
	Cap   float64            `json:"cap,omitempty"`
	Floor float64            `json:"floor,omitempty"`
	Tags  map[string]float64 `json:"tags,omitempty"`
}

// DoctrineLevel is the doctrine's materialisation data: EggShare of a
// presence's bodies are eggs, AdultShare of the hatched ones adults (the rest
// stage 2); a launcher stage stands one Launcher; an exhausted presence
// leaves a Husk stamp of HuskW x HuskH.
type DoctrineLevel struct {
	EggShare   float64 `json:"egg_share"`
	AdultShare float64 `json:"adult_share"`
	Launcher   string  `json:"launcher,omitempty"`
	Husk       string  `json:"husk,omitempty"`
	HuskW      int     `json:"husk_w,omitempty"`
	HuskH      int     `json:"husk_h,omitempty"`
	// Sleeper is the blueprint a dormant presence stands on its level as
	// (phase E, the autonz' sleeper node): nothing else of it is there
	// until it wakes.
	Sleeper string `json:"sleeper,omitempty"`
}

// DoctrineMorph is one body plan's player-facing names.
type DoctrineMorph struct {
	Name     string `json:"name"`     // "Xenos"
	Singular string `json:"singular"` // "xeno", for "a xeno egg"
	Tag      string `json:"tag"`      // "XENO", the Star Map tag's word
	// Blueprint is the prefix of the morph's life-stage blueprints
	// ("xeno": xeno_egg, xeno_hatchling, xeno_stage2, xeno_stage3).
	Blueprint string `json:"blueprint,omitempty"`
}

// DoctrineMobility is how a faction spreads. Only "launcher" is stepped.
type DoctrineMobility struct {
	Kind string `json:"kind"` // launcher | fleet | signal
	// The hive fires at SpreadShare of its site's capacity, never below
	// SpreadMin (and never above the capacity itself), so a cramped site
	// fires rarely rather than never (FireThreshold).
	SpreadShare float64 `json:"spread_share"`
	SpreadMin   int     `json:"spread_min"`
	// CrowdedCapacity: on a site carrying less, the cooldown stretches by
	// CrowdedCapacity / capacity, so a cramped hive fires rarely (Cooldown).
	CrowdedCapacity int     `json:"crowded_capacity,omitempty"`
	BaseRange       float64 `json:"base_range"` // star-map units, the FuelCost metric
	// PylonRange is added to a signal's range per relay pylon standing, up
	// to the stage's relay_pylon count (the autonz; SignalRange).
	PylonRange float64 `json:"pylon_range,omitempty"`
	Cooldown   int     `json:"cooldown_days"`
	Payload    string  `json:"payload"` // the egg's blueprint (display)
	// ShotPop is the population one egg costs the hive; it lands as an
	// EggPop presence.
	ShotPop int `json:"shot_pop"`
	EggPop  int `json:"egg_pop"`
	// EggSpeed is star-map units an egg covers per day (at least one day).
	EggSpeed float64 `json:"egg_speed"`
	// A signal (kind "signal", the autonz) founds a dormant presence of
	// EggPop at a site within range that wakes WakeDays later; it fires on
	// the cooldown once the population reaches the firing threshold or the
	// conversion SignalConversion, from a stage marked Signal.
	WakeDays         int     `json:"wake_days,omitempty"`
	SignalConversion float64 `json:"signal_conversion,omitempty"`
	// A ship (kind "fleet") stays StayDays on a site, then lifts off for one
	// within BaseRange at ShipSpeed units a day.
	StayDays  int     `json:"stay_days,omitempty"`
	ShipSpeed float64 `json:"ship_speed,omitempty"`
}

// DoctrineGrowth is the daily growth rule.
type DoctrineGrowth struct {
	PerDay float64 `json:"per_day"` // fraction of population added per day
	// StarvePerDay is the fraction lost per day while over capacity.
	StarvePerDay float64 `json:"starve_per_day"`
}

// DoctrineCapacity is the tag-based carrying capacity (plan open question:
// tags in A, the real resource tally once B has a level to count).
type DoctrineCapacity struct {
	Base  int            `json:"base"`
	Tags  map[string]int `json:"tags"`  // per archetype tag
	Kinds map[string]int `json:"kinds"` // per location kind (station, ...)
	Min   int            `json:"min"`   // floor after every modifier
}

// DoctrineStage is one stage: reached at Pop, or at Share of the site's
// capacity when that is lower (so a small site still reaches its launcher
// stage); Launcher when it can fire.
type DoctrineStage struct {
	Pop   int     `json:"pop"`
	Share float64 `json:"share,omitempty"`
	Name  string  `json:"name"`
	// SeenName is the stage's player-facing name while the faction's
	// identity is hidden (the hive's top stage is "hive"); "" is Name.
	SeenName string `json:"seen_name,omitempty"`
	Launcher bool   `json:"launcher,omitempty"`
	// Signal marks a stage that can send signals (mobility "signal").
	Signal bool `json:"signal,omitempty"`
	// Stamp is the structure script built when a presence of this stage
	// materialises (W x H); Mix its bodies' share per type (a typed
	// doctrine) or crew blueprint (the autonz).
	Stamp string             `json:"stamp,omitempty"`
	W     int                `json:"w,omitempty"`
	H     int                `json:"h,omitempty"`
	Mix   map[string]float64 `json:"mix,omitempty"`
	// Structures is how many of each structure a settlement of this stage
	// stands (the hive's egg clusters, chamber and spire): the director's
	// cap and what materialise places.
	Structures map[string]int `json:"structures,omitempty"`
}

// DoctrineTech is one entry of the doctrine's research walk.
type DoctrineTech struct {
	ID    string  `json:"id"`
	Cost  float64 `json:"cost"`
	Range float64 `json:"range,omitempty"` // added to the launcher range
	// ConversionRate is added to the conversion rate's multiplier (phase E).
	ConversionRate float64 `json:"conversion_rate,omitempty"`
}

var factionDataCache *FactionData
var factionDataPath string

// factionDataErr is the last LoadFactionData's read or parse error.
var factionDataErr error

// LoadFactionData reads FactionsPath (cached per path). A missing file is an
// empty doctrine set: no faction is seeded and nothing steps (tests that do
// not need the data rely on it). The game itself checks the file at start
// (ValidateFactionData), since the player's own doctrine lives in it.
func LoadFactionData() *FactionData {
	if factionDataCache != nil && factionDataPath == FactionsPath {
		return factionDataCache
	}
	fd := &FactionData{Doctrines: map[string]*Doctrine{}}
	factionDataErr = nil
	if b, err := os.ReadFile(FactionsPath); err != nil {
		factionDataErr = err
	} else if err := json.Unmarshal(b, fd); err != nil {
		factionDataErr = fmt.Errorf("%s: %w", FactionsPath, err)
		fmt.Fprintf(os.Stderr, "factions: %v\n", err)
		fd = &FactionData{Doctrines: map[string]*Doctrine{}}
	}
	if fd.Doctrines == nil {
		fd.Doctrines = map[string]*Doctrine{}
	}
	for _, d := range fd.Doctrines {
		if len(d.Assimilation) == 0 && len(d.SecondWalk) > 0 {
			d.Assimilation, d.AssimilationPerDay = d.SecondWalk, d.SecondWalkPerDay
		}
		if d.Expedition == nil && d.ExpeditionFrom != "" {
			if src := fd.Doctrines[d.ExpeditionFrom]; src != nil && src.Expedition != nil {
				cp := *src.Expedition
				d.Expedition = &cp
			}
		}
		if d.Settlement == nil && d.SettlementFrom != "" {
			if src := fd.Doctrines[d.SettlementFrom]; src != nil && src.Settlement != nil {
				cp := *src.Settlement
				d.Settlement = &cp
			}
		}
	}
	factionDataCache, factionDataPath = fd, FactionsPath
	return fd
}

// ValidateFactionData is the game's start-up check of FactionsPath: it is
// read and parses, the default player doctrine is there with its
// settlement (crew, build menu, research: without it the player's ship has
// none), and every stage structure names a build.json entry when that table
// is loaded (StageBuilt would count an unknown key as standing). A nil error
// is a usable file.
func ValidateFactionData(buildable func(string) bool) error {
	fd := LoadFactionData()
	if factionDataErr != nil {
		return factionDataErr
	}
	d := fd.Doctrine(DefaultPlayerDoctrine)
	if d == nil || d.Settlement == nil || !d.Playable {
		return fmt.Errorf("%s: no playable %q doctrine with a settlement", FactionsPath, DefaultPlayerDoctrine)
	}
	if buildable == nil {
		return nil
	}
	keys := make([]string, 0, len(fd.Doctrines))
	for k := range fd.Doctrines {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for i, st := range fd.Doctrines[k].Stages {
			names := make([]string, 0, len(st.Structures))
			for s := range st.Structures {
				names = append(names, s)
			}
			sort.Strings(names)
			for _, s := range names {
				if !buildable(s) {
					return fmt.Errorf("%s: %s stage %d structure %q is not in build.json", FactionsPath, k, i, s)
				}
			}
		}
	}
	return nil
}

// Doctrine returns a doctrine by key, or nil.
func (fd *FactionData) Doctrine(key string) *Doctrine {
	if fd == nil {
		return nil
	}
	return fd.Doctrines[key]
}

// seedable is the sorted list of seedable doctrine keys.
func (fd *FactionData) seedable() []string {
	var out []string
	for k, d := range fd.Doctrines {
		if d.Seedable {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// MorphAt is the morph players would name at loc: until a later phase
// records what was seen on the ground and whether the shared identity is
// known, it follows the site's scenario (morph_by_scenario, else
// default_morph). A doctrine without morphs names itself.
func (d *Doctrine) MorphAt(loc *Location) DoctrineMorph {
	if m := d.Morphs[d.MorphKeyAt(loc)]; m != nil {
		return *m
	}
	return DoctrineMorph{Name: d.Name, Singular: d.Singular, Tag: strings.ToUpper(d.Name)}
}

// MorphKeyAt is the key of the morph MorphAt names ("crawler").
func (d *Doctrine) MorphKeyAt(loc *Location) string {
	key := d.DefaultMorph
	if loc != nil {
		if k, ok := d.MorphByScenario[loc.ScenarioID]; ok {
			key = k
		}
	}
	return key
}

// IsHome reports whether scenarioID's threat is this faction.
func (d *Doctrine) IsHome(scenarioID string) bool {
	for _, s := range d.HomeScenarios {
		if s == scenarioID {
			return true
		}
	}
	return false
}

// HazardFactor is the growth multiplier on a site running scenarioID.
func (d *Doctrine) HazardFactor(scenarioID string) float64 {
	if f, ok := d.Hazard[scenarioID]; ok {
		return f
	}
	return 1
}

// StageName is the display name of stage i.
func (d *Doctrine) StageName(i int) string {
	if i >= 0 && i < len(d.Stages) {
		return d.Stages[i].Name
	}
	return ""
}

// SeenStageName is stage i's player-facing name: its seen_name, else its
// name. Player-facing text in a normal campaign uses it (hidden identity);
// an observer run may show StageName.
func (d *Doctrine) SeenStageName(i int) string {
	if i >= 0 && i < len(d.Stages) && d.Stages[i].SeenName != "" {
		return d.Stages[i].SeenName
	}
	return d.StageName(i)
}

// OrbitAlert is the message posted when the ship reaches orbit of loc and
// sees a presence there. It names the morph only: the stage names ("hive")
// would name the faction while its identity is hidden.
func (d *Doctrine) OrbitAlert(loc *Location) string {
	return fmt.Sprintf("%s presence detected on %s.", d.MorphAt(loc).Name, loc.Name)
}

// LaunchAlert is the message posted when a live hive on loc fires an egg
// while the player is there, named by morph.
func (d *Doctrine) LaunchAlert(loc *Location) string {
	return fmt.Sprintf("Something burst from the %s spire on %s and arced into the sky.", d.MorphAt(loc).Singular, loc.Name)
}

// RaidAlert is the alert when a settlement of this doctrine sends a raid
// against the player on loc (phase D). Named as the doctrine names itself
// to the player: a rival colony is openly one.
func (d *Doctrine) RaidAlert(loc *Location) string {
	if d.RaidAlertText != "" {
		// The doctrine's own line (the greys, phase E4): {name} is the morph
		// seen there, {place} the site.
		return strings.NewReplacer("{name}", d.MorphAt(loc).Name, "{place}", loc.Name).Replace(d.RaidAlertText)
	}
	if len(d.Morphs) > 0 {
		// Hidden identity: a swarm is named by the morph seen there.
		return fmt.Sprintf("%s are swarming toward your colony on %s.", d.MorphAt(loc).Name, loc.Name)
	}
	who := d.Name
	if d.PlayerName != "" {
		who = d.PlayerName
	}
	return fmt.Sprintf("%s raiders are moving on your colony on %s.", who, loc.Name)
}

// CapacityAt is how many bodies loc can carry for this doctrine.
func (d *Doctrine) CapacityAt(loc *Location) int {
	n := d.Capacity.Base
	for _, t := range loc.CapacityTags() {
		n += d.Capacity.Tags[t]
	}
	n += d.Capacity.Kinds[loc.Kind]
	if n < d.Capacity.Min {
		n = d.Capacity.Min
	}
	if n < 1 {
		n = 1
	}
	return n
}

// FireThreshold is the population a hive on a site carrying cap needs to
// fire: SpreadShare of cap, at least SpreadMin, at most cap.
func (d *Doctrine) FireThreshold(cap int) int {
	m := d.Mobility
	n := int(math.Ceil(m.SpreadShare * float64(cap)))
	if n < m.SpreadMin {
		n = m.SpreadMin
	}
	if n > cap {
		n = cap
	}
	if n < 1 {
		n = 1
	}
	return n
}

// CooldownAt is the launcher cooldown in days on a site carrying cap, before
// the faction-count pressure: Cooldown, stretched on a crowded site.
func (d *Doctrine) CooldownAt(cap int) float64 {
	c := float64(d.Mobility.Cooldown)
	if ref := d.Mobility.CrowdedCapacity; cap > 0 && cap < ref {
		c *= float64(ref) / float64(cap)
	}
	return c
}

// stagePop is the population stage i needs on a site carrying cap.
func (d *Doctrine) stagePop(i, cap int) int {
	s := d.Stages[i]
	if s.Share > 0 {
		if n := int(math.Ceil(s.Share * float64(cap))); n < s.Pop {
			return n
		}
	}
	return s.Pop
}

// Range is the launcher range for a presence knowing techs.
func (d *Doctrine) Range(techs []string) float64 {
	r := d.Mobility.BaseRange
	for _, t := range d.Research {
		for _, k := range techs {
			if k == t.ID {
				r += t.Range
			}
		}
	}
	return r
}

// CapacityTags are the archetype tags capacity reads: Tags, else (a save
// charted before tags were kept) the quest tag.
func (l *Location) CapacityTags() []string {
	if len(l.Tags) > 0 {
		return l.Tags
	}
	return tagSlice(l.QuestTag)
}

// ---- options and seeding ----

// GenOptions are the New Expedition choices that shape generation.
type GenOptions struct {
	// Factions is the number of rival faction presences seeded (the "Rival
	// factions" stepper). 0 is today's game: nothing expands.
	Factions int
	// Doctrine is the player's doctrine; "" is DefaultPlayerDoctrine. The
	// hook for a title-screen race choice (phase F).
	Doctrine string
	// Observer makes the run an observer run (observer.go): set by the
	// title screen from observer mode; the game adds the expedition fleet.
	Observer bool
}

// FactionCountRange is the stepper's default and maximum, from
// generation.json (factions_default, factions_max).
func FactionCountRange() (def, max int) {
	cfg := genConfig()
	return cfg.FactionsDefault, cfg.FactionsMax
}

// FactionSiteOK reports whether a faction presence may be put on loc. Never
// the Wreck Site, Home or anything without a site, never the Training Ground,
// and never a first-jump site the player has not reached yet: that landing is
// the player's (the safe start, firstJump). seeding also refuses first-jump
// sites already visited (there are none at generation; the rule is explicit).
func (c *Campaign) FactionSiteOK(loc *Location, seeding bool) bool {
	if loc == nil || !loc.HasSite() || loc.Tutorial || loc.Kind == AdriftKind || loc.Kind == EncounterKind {
		return false
	}
	if c.firstJump(loc) && (seeding || !loc.Visited) {
		return false
	}
	return true
}

// seedFactions places n presences at generation, drawn from the seedable
// doctrines without replacement (refilling once the pool is used, so a
// one-doctrine data file still seeds n hives), each on a different eligible
// site, weighted toward the doctrine's economy tag. Its own random stream:
// seeding never moves another generation draw.
func (c *Campaign) seedFactions(n int) {
	c.FactionCount = n
	c.FactionDay = c.StarDate
	if n <= 0 {
		return
	}
	fd := LoadFactionData()
	// The bag holds the doctrines seeded from the first faction. One drawn
	// from a later count (seed_from) takes one of the first slots; that
	// slot's draw is still made and its site and number kept back, so every
	// later draw lands where (and is seeded as) it did before such doctrines
	// existed. Those doctrines then pick their sites from a stream of their
	// own.
	var pool, extra []string
	for _, k := range fd.seedable() {
		switch from := fd.Doctrine(k).SeedFrom; {
		case from <= 1:
			pool = append(pool, k)
		case from <= n:
			extra = append(extra, k)
		}
	}
	if len(pool) == 0 {
		return
	}
	extra = extra[:min(len(extra), n)]
	rng := rand.New(rand.NewSource(c.Seed*7919 + 104729))
	var bag []string
	taken := map[string]bool{}
	sites := func(d *Doctrine, r *rand.Rand) *Location {
		var cands []*Location
		for _, loc := range c.sortedLocations() {
			if !taken[loc.ID] && c.FactionSiteOK(loc, true) {
				cands = append(cands, loc)
			}
		}
		loc := pickWeighted(r, cands, func(l *Location) float64 { return economyWeight(d, l) })
		if loc != nil {
			taken[loc.ID] = true
		}
		return loc
	}
	place := func(key string, d *Doctrine, loc *Location) {
		pop := d.SeedPopulation
		if pop < 1 {
			pop = 1
		}
		if d.Mobility.Kind == MobilityFleet {
			// A fleet faction starts as a ship landed on its site, and (the
			// greyish) a mothership far from Home.
			if cc := d.CrewCap(0); cc > 0 {
				pop = min(pop, cc)
			}
			c.newShip(key, d, loc, pop, true)
			if d.Ships != nil && d.Ships.Mothership != nil && c.Mothership(key) == nil {
				c.newMothership(key, d, taken)
			}
			return
		}
		p := c.newPresence(key, d, loc, pop)
		c.recordFaction(d.arrivalEvent(), loc, p, d, nil)
	}
	for i := 0; i < n; i++ {
		if len(bag) == 0 {
			bag = append([]string(nil), pool...)
		}
		j := rng.Intn(len(bag))
		key := bag[j]
		bag = append(bag[:j], bag[j+1:]...)
		d := fd.Doctrine(key)
		loc := sites(d, rng)
		if loc == nil {
			return
		}
		if i < len(extra) {
			c.FactionSeq++ // the slot's number stays reserved
			continue
		}
		place(key, d, loc)
	}
	own := rand.New(rand.NewSource(c.Seed*104729 + 7919))
	for _, key := range extra {
		d := fd.Doctrine(key)
		if loc := sites(d, own); loc != nil {
			place(key, d, loc)
		}
	}
}

// newPresence adds a fresh presence of doctrine key on loc.
func (c *Campaign) newPresence(key string, d *Doctrine, loc *Location, pop int) *FactionPresence {
	c.FactionSeq++
	loc.Factions = append(loc.Factions, FactionPresence{
		ID:          fmt.Sprintf("fp%d", c.FactionSeq),
		Faction:     key,
		Doctrine:    key,
		Population:  pop,
		ArrivedDay:  c.FactionDay,
		LastStepDay: c.FactionDay,
		LastShotDay: NeverFired,
		Seed:        c.Seed + int64(c.FactionSeq)*6271,
	})
	p := &loc.Factions[len(loc.Factions)-1]
	p.Stage = d.stageFor(pop, 0, d.CapacityAt(loc))
	d.AddEconomyUnits(p, pop) // a brood (seeded or landed) is all drones
	return p
}

func (d *Doctrine) stageFor(pop, cur, cap int) int {
	for i := range d.Stages {
		if pop >= d.stagePop(i, cap) && i > cur {
			cur = i
		}
	}
	return cur
}

func economyWeight(d *Doctrine, l *Location) float64 {
	w := 1.0
	for _, t := range l.CapacityTags() {
		if t == d.Economy {
			w += 3
		}
		if d.Conversion.Tags[t] > 0 {
			w += 3 // the convert economy prefers what feeds it (ore)
		}
	}
	return w
}

func pickWeighted(rng *rand.Rand, cands []*Location, weight func(*Location) float64) *Location {
	total := 0.0
	for _, l := range cands {
		total += weight(l)
	}
	if total <= 0 {
		return nil
	}
	r := rng.Float64() * total
	for _, l := range cands {
		r -= weight(l)
		if r < 0 {
			return l
		}
	}
	return cands[len(cands)-1]
}

func (c *Campaign) sortedLocations() []*Location {
	out := make([]*Location, 0, len(c.Locations))
	for _, l := range c.Locations {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- the daily step ----

// StepFactions advances every presence and egg from FactionDay to the star
// date, one day at a time. loaded reports locations whose level is in memory
// (live or background-ticking): their presences are skipped (Phase B
// simulates them for real) and eggs bound for them wait in orbit. nil means
// nothing is loaded. Cheap when the day has not changed.
func (c *Campaign) StepFactions(loaded func(id string) bool) {
	for c.FactionDay < c.StarDate {
		c.FactionDay++
		c.stepFactionDay(loaded)
		c.spotFleets()
	}
}

// pressure is the spread multiplier from the faction count.
func (c *Campaign) pressure(fd *FactionData) float64 {
	n := c.FactionCount
	if n < 1 {
		n = 1
	}
	return 1 + fd.PressurePerFaction*float64(n-1)
}

func (c *Campaign) stepFactionDay(loaded func(id string) bool) {
	fd := LoadFactionData()
	day := c.FactionDay
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	rng := rand.New(rand.NewSource(c.Seed*31 + int64(day)*2654435761 + 7))

	// Eggs land first, so a presence founded today grows from tomorrow.
	// Fleets step in place (a docking ship writes onto its mothership); the
	// ships motherships order are built after the pass.
	n := len(c.Fleets)
	gone := make([]bool, n)
	c.shipOrders = nil
	for i := 0; i < n; i++ {
		f := &c.Fleets[i]
		if f.Kind == FleetShip {
			gone[i] = !c.stepShip(rng, fd, f, day, isLoaded)
			continue
		}
		if f.ArriveDay > day || isLoaded(f.ToID) {
			continue
		}
		c.landFleet(fd, *f)
		gone[i] = true
	}
	kept := c.Fleets[:0]
	for i, f := range c.Fleets {
		if i >= n || !gone[i] {
			kept = append(kept, f)
		}
	}
	c.Fleets = kept
	for _, faction := range c.shipOrders {
		c.buildShip(faction, day, isLoaded)
	}
	c.shipOrders = nil
	if len(c.Fleets) == 0 {
		c.Fleets = nil // as a save reads it back
	}

	for _, loc := range c.sortedLocations() {
		if len(loc.Factions) == 0 || isLoaded(loc.ID) {
			continue
		}
		alive := loc.Factions[:0]
		for i := range loc.Factions {
			p := loc.Factions[i]
			d := fd.Doctrine(p.Doctrine)
			if d == nil {
				alive = append(alive, p) // unknown doctrine: keep, do not step
				continue
			}
			if c.stepPresence(rng, fd, d, loc, &p, day, isLoaded) {
				alive = append(alive, p)
			}
		}
		loc.Factions = alive
		if len(loc.Factions) == 0 {
			loc.Factions = nil
		}
	}
}

// stepPresence advances one presence one day; false when it died out.
func (c *Campaign) stepPresence(rng *rand.Rand, fd *FactionData, d *Doctrine, loc *Location, p *FactionPresence, day history.StarDate, isLoaded func(string) bool) bool {
	p.LastStepDay = day
	// A signal's dormant presence sleeps until its wake day (phase E).
	if !c.stepDormant(loc, p, d, day) {
		return true
	}
	// 1. Grow toward capacity; starve above it; the scenario's hazard scales
	// (or reverses) the rate. Under the convert economy the capacity is the
	// share converted (ConvertCap), and a fully converted site stops growing.
	cap := d.ConvertCap(loc, p)
	rate := d.Growth.PerDay * d.HazardFactor(loc.ScenarioID)
	var delta float64
	switch {
	case p.Population > cap:
		delta = -d.Growth.StarvePerDay * float64(p.Population)
	case d.FullyConverted(p):
		delta = 0
	default:
		delta = rate * float64(p.Population)
	}
	before := p.Population
	p.Growth += delta
	for p.Growth >= 1 {
		p.Population++
		p.Growth--
	}
	for p.Growth <= -1 {
		p.Population--
		p.Growth++
	}
	if p.Population >= cap && delta > 0 {
		p.Population, p.Growth = cap, 0
	}
	if p.Population <= 0 {
		// Died out on its own (a hazard, starvation): it leaves a husk.
		c.exhaust(loc, p, d)
		return false
	}
	var born map[string]int
	if d.HasTypes() && p.Population > before {
		// The types the new bodies went to (the growth rule), for the tally.
		was := map[string]int{}
		for k, v := range p.Units {
			was[k] = v
		}
		d.NormalizeUnits(p, loc)
		born = map[string]int{}
		for k, v := range p.Units {
			if v > was[k] {
				born[k] = v - was[k]
			}
		}
	} else {
		d.NormalizeUnits(p, loc)
	}
	// 2. Work: the convert economy converts (phase E); the biomass economy
	// has no abstract tally.
	if d.Converts() {
		c.convertWork(d, loc, p)
	}
	c.hiveTally(d, loc, p, born) // the hive's larvae, biomass and resin
	// 3. Stage up; a lost launcher regrows after the cooldown.
	c.stageUp(loc, p, d, d.CapacityAt(loc))
	if p.LauncherLost && int(day-p.LauncherLostDay) >= d.Mobility.Cooldown {
		p.LauncherLost = false
	}
	// 4. Research: the spine's next unknown entry, and the second walk's
	// (the autonz' assimilation) on its own points.
	if next := nextUnknown(d.Research, p.KnownTechs); p.Stage > 0 && next != nil {
		p.ResearchPts += d.ResearchPerDay * float64(p.Stage)
		if p.ResearchPts >= next.Cost {
			p.ResearchPts -= next.Cost
			p.KnownTechs = append(p.KnownTechs, next.ID)
		}
	}
	if next := nextUnknown(d.Assimilation, p.KnownTechs); next != nil && len(d.AssimilationPerDay) > 0 {
		p.AssimPoints += d.AssimilationPerDay[min(max(p.Stage, 0), len(d.AssimilationPerDay)-1)]
		if p.AssimPoints >= next.Cost {
			p.AssimPoints -= next.Cost
			p.KnownTechs = append(p.KnownTechs, next.ID)
		}
	}
	// The structures a building settlement stands (Tier 1 bookkeeping).
	if d.Builds() {
		p.Built = tier1Rebuild(p.Built, d.StageBuilt(p.Stage, p.KnownTechs))
	}
	// 5. Spread.
	switch d.Mobility.Kind {
	case "launcher":
		c.tryLaunch(rng, fd, d, loc, p, day, isLoaded)
	case MobilitySignal:
		c.trySignal(rng, fd, d, loc, p, day, isLoaded, -1)
	}
	return true
}

// nextUnknown is walk's first entry techs do not know, nil when all are.
func nextUnknown(walk []DoctrineTech, techs []string) *DoctrineTech {
	for i := range walk {
		if !hasStr(techs, walk[i].ID) {
			return &walk[i]
		}
	}
	return nil
}

// tryLaunch fires an egg when the hive is big enough, its stage has a
// launcher and the cooldown has passed. The target is an eligible site in
// range with no presence of this faction and no egg already bound for it,
// weighted toward the economy tag; with none, nothing fires and the cooldown
// is not spent.
func (c *Campaign) tryLaunch(rng *rand.Rand, fd *FactionData, d *Doctrine, loc *Location, p *FactionPresence, day history.StarDate, isLoaded func(string) bool) bool {
	m := d.Mobility
	cands := c.launchCands(fd, d, loc, p, day, isLoaded)
	if cands == nil {
		return false
	}
	target := pickWeighted(rng, cands, func(l *Location) float64 { return economyWeight(d, l) })
	if target == nil {
		return false
	}
	shot := m.ShotPop
	if shot < 1 {
		shot = 1
	}
	egg := m.EggPop
	if egg < 1 {
		egg = 1
	}
	d.NormalizeUnits(p, loc) // the record as it stood, then the shot
	p.Population -= shot
	if p.Population < 1 {
		p.Population = 1
	}
	if d.HasTypes() {
		d.ShotUnits(p, loc, p.UnitsTotal()-p.Population)
	}
	p.LastShotDay = day
	speed := m.EggSpeed
	if speed <= 0 {
		speed = 1
	}
	days := int(math.Ceil(math.Hypot(target.X-loc.X, target.Y-loc.Y) / speed))
	if days < 1 {
		days = 1
	}
	c.FleetSeq++
	c.Fleets = append(c.Fleets, Fleet{
		ID: fmt.Sprintf("fl%d", c.FleetSeq), Kind: FleetProjectile,
		FromID: loc.ID, ToID: target.ID, FromX: loc.X, FromY: loc.Y, ToX: target.X, ToY: target.Y,
		LaunchDay: day, ArriveDay: day + history.StarDate(days),
		Presence: FactionPresence{
			Faction: p.Faction, Doctrine: p.Doctrine, Population: egg,
			KnownTechs: append([]string(nil), p.KnownTechs...),
		},
		// Seen leaving a hive the player knows.
		Known: p.Known,
	})
	c.recordFaction("faction_launched", loc, p, d, nil)
	return true
}

// launchCands is the sites a launcher of p would fire at on day: nil when it
// would not fire (under the threshold, not a launcher stage, its launcher
// lost, the cooldown not passed, or no free site in range). It draws no
// random numbers.
func (c *Campaign) launchCands(fd *FactionData, d *Doctrine, loc *Location, p *FactionPresence, day history.StarDate, isLoaded func(string) bool) []*Location {
	m := d.Mobility
	cap := d.CapacityAt(loc)
	if p.Population < d.FireThreshold(cap) || p.Stage >= len(d.Stages) || !d.Stages[p.Stage].Launcher || p.LauncherLost {
		return nil
	}
	// A cramped site's first shot also waits a stretched cooldown from its
	// arrival; elsewhere the first shot waits only on growth.
	since := p.LastShotDay
	if since == NeverFired && cap < m.CrowdedCapacity {
		since = p.ArrivedDay
	}
	cool := int(math.Ceil(d.CooldownAt(cap) / c.pressure(fd)))
	if since != NeverFired && int(day-since) < cool {
		return nil
	}
	rng2 := d.Range(p.KnownTechs)
	inbound := map[string]bool{}
	for _, f := range c.Fleets {
		if f.Presence.Faction == p.Faction {
			inbound[f.ToID] = true
		}
	}
	var cands []*Location
	for _, t := range c.sortedLocations() {
		if t == loc || inbound[t.ID] || isLoaded(t.ID) || !c.FactionSiteOK(t, false) || t.HasFaction(p.Faction) {
			continue
		}
		if math.Hypot(t.X-loc.X, t.Y-loc.Y) <= rng2 {
			cands = append(cands, t)
		}
	}
	return cands
}

// LaunchReady reports whether presence id on loaded loc would fire today
// with pop bodies (the Tier 1 rule; the hive's launch rung walks its shot's
// bodies to the spire then). No state changes.
func (c *Campaign) LaunchReady(loc *Location, id string, pop int, loaded func(string) bool) bool {
	p := loc.PresenceByID(id)
	if p == nil || !p.Materialized || pop <= 0 {
		return false
	}
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	if d == nil || d.Mobility.Kind != "launcher" {
		return false
	}
	q := *p
	q.Population = pop
	q.Stage = d.stageFor(pop, q.Stage, d.CapacityAt(loc))
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	return len(c.launchCands(fd, d, loc, &q, c.FactionDay, isLoaded)) > 0
}

// LiveLaunch is the launcher rule for a presence on a loaded level (live or
// background-ticking), which Tier 1 skips: the world manager calls it once a
// day with what the level holds (pop living bodies, whether the launcher
// stands). The record takes the live count (its stage rising with it) and
// fires by exactly the Tier 1 rule (threshold, launcher stage, the same
// LastShotDay cooldown, a free target in range), from the standing launcher
// only. It returns the fleet fired, or nil; the caller removes the shot's
// bodies from the level. Its own random stream per presence and day.
func (c *Campaign) LiveLaunch(loc *Location, id string, pop int, launcher bool, loaded func(string) bool) *Fleet {
	p := loc.PresenceByID(id)
	if p == nil || !p.Materialized || !launcher || pop <= 0 {
		return nil
	}
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	if d == nil || d.Mobility.Kind != "launcher" {
		return nil
	}
	day := c.FactionDay
	p.Population, p.Growth = pop, 0
	c.stageUp(loc, p, d, d.CapacityAt(loc))
	d.NormalizeUnits(p, loc)
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	rng := rand.New(rand.NewSource(c.Seed*37 + int64(day)*2654435761 + p.Seed))
	if !c.tryLaunch(rng, fd, d, loc, p, day, isLoaded) {
		return nil
	}
	return &c.Fleets[len(c.Fleets)-1]
}

// landFleet founds (or reinforces) a presence where an egg came down. A target
// that became ineligible meanwhile (the player reached a first-jump site, a
// hive formed there) takes the egg as reinforcement or loses it.
func (c *Campaign) landFleet(fd *FactionData, f Fleet) {
	loc := c.Locations[f.ToID]
	e := f.Presence
	d := fd.Doctrine(e.Doctrine)
	if loc == nil || d == nil || !c.FactionSiteOK(loc, false) {
		return
	}
	if p := loc.Presence(e.Faction); p != nil {
		d.NormalizeUnits(p, loc)
		p.Population += e.Population
		d.AddEconomyUnits(p, e.Population) // a landed brood is all drones
		return
	}
	// A brood: built onto the level the next time it is (materialise).
	p := c.newPresence(e.Faction, d, loc, e.Population)
	p.KnownTechs = append([]string(nil), e.KnownTechs...)
	if loc.ID == c.CurrentLocationID {
		p.Known = true // landed under the ship's nose
	}
	c.recordFaction("faction_arrived", loc, p, d, nil)
}

// stageUp raises p to the highest stage its population reaches on a site
// carrying cap, recording faction_grew.
func (c *Campaign) stageUp(loc *Location, p *FactionPresence, d *Doctrine, cap int) {
	if s := d.stageFor(p.Population, p.Stage, cap); s > p.Stage {
		p.Stage = s
		c.recordFaction("faction_grew", loc, p, d, map[string]string{"stage": d.SeenStageName(s)})
	}
}

// exhaust records a presence that died out on its own and leaves its husk on
// the site (stamped by the level the next time it is built). The caller
// drops the presence.
func (c *Campaign) exhaust(loc *Location, p *FactionPresence, d *Doctrine) {
	c.recordFaction("faction_exhausted", loc, p, d, nil)
	loc.Husks = append(loc.Husks, FactionHusk{
		Presence: p.ID, Faction: p.Faction, Doctrine: p.Doctrine,
		Morph: d.MorphKeyAt(loc), Day: c.FactionDay, Seed: p.Seed + 1,
	})
}

// ReconcilePresence counts a level back into presence id on loc as the
// level freezes or saves (Tier 2): pop is its living bodies there and
// launcher whether its launcher stands. Population takes the count and the
// stage rises with it (never falls: the structures stay); a launcher stage
// that lost its launcher cannot fire until it regrows. Zero bodies is a
// wipe (the player cleared it, faction_wiped): the presence is dropped and
// true returned. A presence not yet built onto the level is left alone.
func (c *Campaign) ReconcilePresence(loc *Location, id string, pop int, launcher bool) (wiped bool) {
	p := loc.PresenceByID(id)
	if p == nil || !p.Materialized {
		return false
	}
	d := LoadFactionData().Doctrine(p.Doctrine)
	if d == nil {
		return false
	}
	day := c.StarDate
	if pop <= 0 {
		// The player was there to see it.
		p.Known = true
		c.recordFaction("faction_wiped", loc, p, d, nil)
		loc.RemovePresence(id)
		return true
	}
	p.Population, p.Growth = pop, 0
	c.stageUp(loc, p, d, d.CapacityAt(loc))
	d.NormalizeUnits(p, loc)
	switch {
	case launcher:
		p.LauncherLost, p.LauncherStood = false, true
	case p.Stage < len(d.Stages) && d.Stages[p.Stage].Launcher && p.LevelStage >= p.Stage && !p.LauncherLost && p.LauncherStood:
		p.LauncherLost, p.LauncherLostDay, p.LauncherStood = true, day, false
	}
	return false
}

// ReconcileUnits takes a typed presence's living bodies by type from its
// level (eggs as their type, larvae not counted) as its Units, before
// ReconcilePresence takes the count. Untyped bodies (a type the doctrine does
// not have) are left to the count to fill by the growth rule.
func (c *Campaign) ReconcileUnits(loc *Location, id string, units map[string]int) {
	p := loc.PresenceByID(id)
	if p == nil || !p.Materialized {
		return
	}
	d := LoadFactionData().Doctrine(p.Doctrine)
	if !d.HasTypes() {
		return
	}
	p.Units = map[string]int{}
	for k, v := range units {
		if d.Types[k] != nil && v > 0 {
			p.Units[k] = v
		}
	}
	if len(p.Units) == 0 {
		p.Units = nil
	}
}

// PresenceByID returns loc's presence with id, or nil.
func (l *Location) PresenceByID(id string) *FactionPresence {
	for i := range l.Factions {
		if l.Factions[i].ID == id {
			return &l.Factions[i]
		}
	}
	return nil
}

// RemovePresence drops presence id from loc.
func (l *Location) RemovePresence(id string) {
	for i := range l.Factions {
		if l.Factions[i].ID == id {
			l.Factions = append(l.Factions[:i], l.Factions[i+1:]...)
			break
		}
	}
	if len(l.Factions) == 0 {
		l.Factions = nil
	}
}

// NeverFired is LastShotDay before a presence's first shot.
const NeverFired history.StarDate = -1

// Presence returns loc's presence of faction, or nil.
func (l *Location) Presence(faction string) *FactionPresence {
	for i := range l.Factions {
		if l.Factions[i].Faction == faction {
			return &l.Factions[i]
		}
	}
	return nil
}

// HasFaction reports whether loc holds a presence of faction.
func (l *Location) HasFaction(faction string) bool { return l.Presence(faction) != nil }

// KnownPresences are the presences on loc the player has seen.
func (l *Location) KnownPresences() []*FactionPresence {
	var out []*FactionPresence
	for i := range l.Factions {
		if l.Factions[i].Known {
			out = append(out, &l.Factions[i])
		}
	}
	return out
}

// ---- live history and what the player knows ----

// recordFaction writes a faction event to the live history, dated on the
// faction clock's day (the star date when stepped live). An event about a
// presence the player has not seen is recorded unknown and kept on the
// presence, revealed with it (RevealPresences).
func (c *Campaign) recordFaction(typ string, loc *Location, p *FactionPresence, d *Doctrine, vars map[string]string) string {
	// Log lines name the morph, never the faction (hidden identity).
	m := d.MorphAt(loc)
	v := map[string]string{"faction": m.Name, "singular": m.Singular}
	for k, val := range vars {
		v[k] = val
	}
	if c.History == nil {
		return ""
	}
	site := ""
	if loc != nil {
		site = loc.History
		v["place"] = loc.Name
	}
	// Beside the site's history, never in it: the layers are independent.
	id := c.History.RecordBeside(typ, c.FactionDay, site, v)
	if p == nil || p.Known {
		return id
	}
	delete(c.History.Known, id)
	if c.Observer {
		return id // an observer run shows every faction event: nothing to reveal later
	}
	p.Events = append(p.Events, id)
	if n := len(p.Events); n > maxPresenceEvents {
		// The oldest stay in the history, unknown: a presence seen after a
		// long time unseen reveals its recent past, not all of it.
		p.Events = append(p.Events[:0], p.Events[n-maxPresenceEvents:]...)
	}
	return id
}

// maxPresenceEvents caps the unseen events a presence keeps to reveal.
const maxPresenceEvents = 64

// RevealPresences makes every presence on loc known (the ship is in orbit or
// its scanner swept it) and their recorded events with them. It returns the
// presences newly revealed.
func (c *Campaign) RevealPresences(loc *Location) []*FactionPresence {
	if loc == nil {
		return nil
	}
	var out []*FactionPresence
	// A ship landed here is seen from orbit too.
	for _, f := range c.LandedFleets(loc.ID) {
		if !f.Known {
			f.Known = true
			out = append(out, &f.Presence)
		}
	}
	for i := range loc.Factions {
		p := &loc.Factions[i]
		if p.Known {
			continue
		}
		p.Known = true
		if c.History != nil {
			for _, ev := range p.Events {
				c.History.MarkKnown(ev)
			}
		}
		p.Events = nil
		out = append(out, p)
		// Its eggs still in flight are seen leaving it.
		for j := range c.Fleets {
			if f := &c.Fleets[j]; f.FromID == loc.ID && f.Presence.Faction == p.Faction {
				f.Known = true
			}
		}
	}
	return out
}

// RevealPresencesWithin reveals the presences on every discovered site within
// r of (x, y): a scanner sweep.
func (c *Campaign) RevealPresencesWithin(x, y, r float64) []*FactionPresence {
	c.revealFleetsWithin(x, y, r)
	var out []*FactionPresence
	for _, loc := range c.sortedLocations() {
		if loc.Discovered && math.Hypot(loc.X-x, loc.Y-y) <= r {
			out = append(out, c.RevealPresences(loc)...)
		}
	}
	return out
}

// ---- save migration ----

// MigrateCampaign brings a loaded campaign up to CampaignVersion. Version 2
// (before factions) gains an empty faction layer: no presences, no rival
// count, the faction clock at the star date, and capacity tags from the quest
// tag. Version 3 (techs on the campaign) gains the player's presence, running
// the default doctrine, holding the save's known techs. Anything older is
// refused.
func MigrateCampaign(c *Campaign) error {
	if c.Version < 2 {
		return fmt.Errorf("save version %d, need %d", c.Version, CampaignVersion)
	}
	if c.Version == 2 {
		c.FactionDay = c.StarDate
		for _, loc := range c.Locations {
			if len(loc.Tags) == 0 && loc.QuestTag != "" {
				loc.Tags = []string{loc.QuestTag}
			}
		}
		c.Version = 3
	}
	if c.Version == 3 {
		if c.Player == nil {
			c.Player = NewPlayerPresence("")
		}
		c.Player.KnownTechs = append(c.Player.KnownTechs, c.LegacyKnownTechs...)
		c.LegacyKnownTechs = nil
		c.Version = 4
	}
	c.migrateTechAliases()
	return nil
}

// migrateTechAliases renames a doctrine's aliased techs on every presence and
// egg in flight (a Phase B hive's tier_advanced is its warrior brood, at the
// same place in the walk), in place: additive, so the version stays 4.
func (c *Campaign) migrateTechAliases() {
	fd := LoadFactionData()
	fix := func(p *FactionPresence) {
		d := fd.Doctrine(p.Doctrine)
		if d == nil || len(d.TechAliases) == 0 {
			return
		}
		for i, k := range p.KnownTechs {
			if a, ok := d.TechAliases[k]; ok {
				p.KnownTechs[i] = a
			}
		}
	}
	for _, loc := range c.Locations {
		for i := range loc.Factions {
			fix(&loc.Factions[i])
		}
	}
	for i := range c.Fleets {
		fix(&c.Fleets[i].Presence)
	}
}

// tier1Rebuild is a building settlement's Tier 1 day of structures: unknown
// (nil) takes the stage's set at once; otherwise one structure missing from
// the stage's set is rebuilt (a razed settlement grows back a structure a
// day, not all at once), and structures the stage no longer holds go.
func tier1Rebuild(built, want map[string]int) map[string]int {
	if built == nil {
		return want
	}
	out := map[string]int{}
	for k, n := range want {
		out[k] = min(built[k], n)
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if out[k] < want[k] {
			out[k]++
			break
		}
	}
	for k, n := range out {
		if n == 0 {
			delete(out, k)
		}
	}
	return out
}
