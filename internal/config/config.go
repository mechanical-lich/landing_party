package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	TileSizeW    int `json:"tileSizeW"`
	TileSizeH    int `json:"tileSizeH"`
	SpriteSizeW  int `json:"spriteSizeW"`
	SpriteSizeH  int `json:"spriteSizeH"`
	WorldWidth   int `json:"worldWidth"`
	WorldHeight  int `json:"worldHeight"`
	ScreenWidth  int `json:"screenWidth"`
	ScreenHeight int `json:"screenHeight"`
	// Starting Z lives on each level via its SurfaceZ (set by the terrain
	// primer) — planet maps put it at the regolith surface, station maps put
	// it at the top floor (0). No global default.
	Title                  string `json:"title"`
	DPI                    int    `json:"dpi"`
	BlueprintPath          string `json:"blueprintPath"`
	RenderPathfindingSteps bool   `json:"renderPathfindingSteps"`
	DebugDisableLighting   bool   `json:"debugDisableLighting"`
	DebugDisableLookdown   bool   `json:"debugDisableLookdown"`
	DebugShowAutotileMask  bool   `json:"debugShowAutotileMask"`
	DebugShowScenarioID    bool   `json:"debugShowScenarioID"`
	// UnlockAllResearch, when true, seeds every tech in data/research.json
	// into the campaign's KnownTechs on creation. Debug aid for testing
	// research-gated features (Encyclopedia, Global Inventory tiers, etc.)
	// without grinding research time.
	UnlockAllResearch      bool   `json:"unlockAllResearch"`
	ProfileCPU             bool   `json:"profileCPU"`
	ProfileMemory          bool   `json:"profileMemory"`
	OverrideGCMemoryLimit  int64  `json:"overrideGCMemoryLimit"`
	// Rogue mode key-repeat: frames before held WASD starts repeating, and the
	// interval between repeated moves. At 60 fps, 20 delay ≈ 333 ms, 4 interval ≈ 67 ms.
	RogueKeyRepeatDelay    int `json:"rogueKeyRepeatDelay"`
	RogueKeyRepeatInterval int `json:"rogueKeyRepeatInterval"`
	RogueAutoMoveInterval  int `json:"rogueAutoMoveInterval"`

	// Audio bus volumes, 0..1. Base defaults live in data/config.json; the
	// settings screen writes player overrides to config.local.json.
	UIVolume    float64 `json:"uiVolume"`
	GameVolume  float64 `json:"gameVolume"`
	MusicVolume float64 `json:"musicVolume"`

	// Footstep audio toggles. When off, footsteps of that side are silent but
	// still emit their sound event (AI hearing is unaffected).
	FriendlyFootsteps bool `json:"friendlyFootsteps"`
	EnemyFootsteps    bool `json:"enemyFootsteps"`
}

// LoadConfig reads and unmarshals a JSON config file.
func LoadConfig(filePath string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
		return nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
		return nil, err
	}
	return &cfg, nil
}

// localConfigPath is the gitignored per-developer override file.
const localConfigPath = "data/config.local.json"

// applyLocalOverrides merges config.local.json on top of cfg if the file
// exists. Only the fields present in the local file are overwritten; all
// others keep their value from the base config.
func applyLocalOverrides(cfg *Config) {
	data, err := os.ReadFile(localConfigPath)
	if os.IsNotExist(err) {
		return // no local overrides, nothing to do
	}
	if err != nil {
		log.Printf("config: could not read %s: %v (ignoring)", localConfigPath, err)
		return
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		log.Printf("config: could not parse %s: %v (ignoring)", localConfigPath, err)
		return
	}
	log.Printf("config: applied local overrides from %s", localConfigPath)
}

var settings *Config

func Global() *Config {
	if settings == nil {
		var err error
		settings, err = LoadConfig("data/config.json")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		applyLocalOverrides(settings)
	}
	return settings
}

// Reload discards the cached config so the next Global() re-reads data/config.json
// and re-applies config.local.json — used after the settings screen writes an
// override.
func Reload() {
	settings = nil
	Global()
}

// mergeAudioOverrides layers the audio settings onto whatever config.local.json
// already contains, so writing them never drops other local settings. Pure (no
// IO) so it's unit-testable.
func mergeAudioOverrides(existing []byte, ui, game, music float64, friendlyFoot, enemyFoot bool) ([]byte, error) {
	overrides := map[string]any{}
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &overrides) // best effort; a bad file is replaced
	}
	overrides["uiVolume"] = ui
	overrides["gameVolume"] = game
	overrides["musicVolume"] = music
	overrides["friendlyFootsteps"] = friendlyFoot
	overrides["enemyFootsteps"] = enemyFoot
	return json.MarshalIndent(overrides, "", "  ")
}

// SaveAudioSettings writes the audio settings into config.local.json (preserving
// other overrides) and reloads the global config.
func SaveAudioSettings(ui, game, music float64, friendlyFoot, enemyFoot bool) error {
	existing, _ := os.ReadFile(localConfigPath) // missing file → empty, fine
	out, err := mergeAudioOverrides(existing, ui, game, music, friendlyFoot, enemyFoot)
	if err != nil {
		return err
	}
	if err := os.WriteFile(localConfigPath, out, 0o644); err != nil {
		return err
	}
	Reload()
	return nil
}
