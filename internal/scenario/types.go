package scenario

import "github.com/mechanical-lich/scifi_settlements/internal/wincondition"

type SpawnRule struct {
	SpawnRate int      `json:"spawn_rate"`
	LightMin  int      `json:"light_min"`
	LightMax  int      `json:"light_max"`
	Tiles     []string `json:"tiles"`
	MinZ      int      `json:"min_z"`
	MaxZ      int      `json:"max_z"`
}

type Scenario struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Enabled       bool                    `json:"enabled"`
	HostileMax    int                     `json:"hostile_max"`
	SpawnRules    map[string]SpawnRule    `json:"spawn_rules"`
	SetupScripts  []string                `json:"setup_scripts"`
	WinConditions wincondition.RuleSet    `json:"win_conditions"`
}
