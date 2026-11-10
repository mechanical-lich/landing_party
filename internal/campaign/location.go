package campaign

// Location is a descriptor for one node on the space overworld. It never holds
// a live *world.Level — the level is either generated on demand from
// MapID/ScenarioID/Seed or deserialized from SaveFile (a paused, previously
// visited location).
type Location struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"` // planet, moon, asteroid_field, ...
	MapID      string `json:"map_id"`
	ScenarioID string `json:"scenario_id"`
	// History is the SiteHistory ID bound at charting (see Campaign.History);
	// "" for Home, the Wreck Site and the tutorial.
	History string `json:"history,omitempty"`
	// Tier is the site history's danger class ("threat", "hazard", "quiet"),
	// shown on the Star Map from charting; "" for a site without a history.
	// Saves from before tiers derive it (Campaign.LocationTier).
	Tier string `json:"tier,omitempty"`
	Seed int64  `json:"seed"`

	Discovered bool `json:"discovered"`
	Visited    bool `json:"visited"`

	// X, Y are the location's position on the star map. Fuel cost to travel is
	// the distance between the ship's current location and the destination
	// (see Campaign.FuelCost).
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Summary string  `json:"summary"`

	// QuestTag carries the location's first archetype tag, used to match
	// tag-gated quest templates posted to an existing system.
	QuestTag string `json:"quest_tag,omitempty"`
	// Tags are the archetype's tags (ore, biomass, hostile, salvage, ...),
	// read by faction carrying capacity. Saves from before factions carry
	// only QuestTag (see CapacityTags).
	Tags []string `json:"tags,omitempty"`

	// Tutorial marks the Training Ground: random contracts are never posted to
	// it (see MaybeTravelQuests).
	Tutorial bool `json:"tutorial,omitempty"`

	// SaveFile is "" until the location has been visited and frozen; once set
	// it is the relative path of the per-location level save inside the
	// campaign directory.
	SaveFile string `json:"save_file,omitempty"`

	// Loaded records that this location was in memory when the campaign was saved,
	// staffed and simulating, so loading the campaign brings it back the same way.
	Loaded bool `json:"loaded,omitempty"`

	// SettlementName is this location's own colony settlement, the key into the
	// global settlement map. Assigned when the location's level is first built.
	SettlementName string `json:"settlement_name,omitempty"`

	// CameraX/Y/Z and BuildMode capture the player's view state so a resumed
	// location reopens exactly where they left it.
	CameraX   int    `json:"camera_x,omitempty"`
	CameraY   int    `json:"camera_y,omitempty"`
	CameraZ   int    `json:"camera_z,omitempty"`
	BuildMode string `json:"build_mode,omitempty"`

	// Colonists is the number of living colonists left here, recorded on
	// Freeze. Used for campaign-wide total-wipe detection.
	Colonists int `json:"colonists,omitempty"`

	// Fixtures are quest-placed things (named bosses, bounty creatures, loot)
	// to materialize into this location's level. Spawned idempotently on every
	// level load (see the game-side spawn hook); persisted so a fixture added
	// after the location was first visited still appears on the next trip.
	Fixtures []QuestFixture `json:"fixtures,omitempty"`
	// Boarding is a boarded faction ship (Kind "encounter", phase D).
	Boarding *Boarding `json:"boarding,omitempty"`
	// Ruins are the camps fleet factions left here when they lifted off,
	// ruined on the level live or at its next build (phase D4).
	Ruins []FactionRuin `json:"ruins,omitempty"`

	// StorageSummary is the cached blueprint→total-quantity snapshot of all
	// colony-owned storage at this location, refreshed on Freeze. Used by the
	// Global Inventory modal to display data from parked sites without
	// loading the full level.
	StorageSummary map[string]int `json:"storage_summary,omitempty"`

	// MapW, MapH are the site's generated map dimensions, cached once the
	// location has been built so the Star Map can size its icon without loading
	// the level. Zero until the site has been generated (unvisited).
	MapW int `json:"map_w,omitempty"`
	MapH int `json:"map_h,omitempty"`

	// StationFootprint is a downsampled occupancy grid (one row bitmask per
	// entry) of a station's structure, captured at generation so the Star Map
	// can draw a map-shaped icon. Only set for station-kind locations.
	StationFootprint []uint16 `json:"station_footprint,omitempty"`

	// Factions are the enemy presences on this site (faction.go), on top of
	// its history: they never change the chapter or scenario.
	Factions []FactionPresence `json:"factions,omitempty"`
	// Husks are presences that died out here on their own (exhausted), to
	// be left on the level as a husk stamp the next time it is built.
	Husks []FactionHusk `json:"husks,omitempty"`
}

// QuestFixture describes one quest-placed entity for a location.
type QuestFixture struct {
	QuestID   string `json:"quest_id"`
	Blueprint string `json:"blueprint"`
	Name      string `json:"name,omitempty"`
	Kind      string `json:"kind,omitempty"` // "boss" | "loot" | ...
	Spawned   bool   `json:"spawned,omitempty"`

	// Structure, when set, names a generator script run via gen_structure at
	// the fixture's chosen tile before the entity spawns — the boss/loot lands
	// inside a generated building instead of in the open. A fixture with a
	// Structure but no Blueprint is flavor: it builds the structure only.
	Structure string `json:"structure,omitempty"`
	StructW   int    `json:"struct_w,omitempty"`
	StructH   int    `json:"struct_h,omitempty"`

	// EventID is the history event a datapad records. A "lore_datapad" fixture
	// carries only that (no quest) and is placed at the event's resolved
	// origin; a quest datapad carries its quest and the site's story event.
	EventID string `json:"event_id,omitempty"`
}

// AddFixture appends a fixture for this location unless one already exists for
// the same quest.
func (l *Location) AddFixture(f QuestFixture) {
	for i := range l.Fixtures {
		if l.Fixtures[i].QuestID == f.QuestID && l.Fixtures[i].EventID == f.EventID {
			return
		}
	}
	l.Fixtures = append(l.Fixtures, f)
}
