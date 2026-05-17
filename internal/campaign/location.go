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
	Seed       int64  `json:"seed"`

	Discovered bool `json:"discovered"`
	Visited    bool `json:"visited"`

	// X, Y are the location's position on the star map. Fuel cost to travel is
	// derived from the distance between the ship's current location and the
	// destination (see Campaign.FuelCost). FuelCost is a legacy fallback used
	// only when coordinates are absent.
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	FuelCost int     `json:"fuel_cost"`
	Summary  string  `json:"summary"`

	// RevealQuest, when non-empty, names the quest whose completion flips
	// Discovered to true (Phase 2 wiring).
	RevealQuest string `json:"reveal_quest,omitempty"`
	// QuestTag is an opaque marker used by quest setup scripts.
	QuestTag string `json:"quest_tag,omitempty"`

	// SaveFile is "" until the location has been visited and frozen; once set
	// it is the relative path of the per-location level save inside the
	// campaign directory.
	SaveFile string `json:"save_file,omitempty"`

	// CameraX/Y/Z and BuildMode capture the player's view state so a resumed
	// location reopens exactly where they left it.
	CameraX   int    `json:"camera_x,omitempty"`
	CameraY   int    `json:"camera_y,omitempty"`
	CameraZ   int    `json:"camera_z,omitempty"`
	BuildMode string `json:"build_mode,omitempty"`
}
