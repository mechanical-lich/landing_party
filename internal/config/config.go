package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	TileSizeW              int    `json:"tileSizeW"`
	TileSizeH              int    `json:"tileSizeH"`
	SpriteSizeW            int    `json:"spriteSizeW"`
	SpriteSizeH            int    `json:"spriteSizeH"`
	WorldWidth             int    `json:"worldWidth"`
	WorldHeight            int    `json:"worldHeight"`
	ScreenWidth            int    `json:"screenWidth"`
	ScreenHeight           int    `json:"screenHeight"`
	WorldGenSizeW          int    `json:"worldGenSizeW"`
	WorldGenSizeH          int    `json:"worldGenSizeH"`
	WorldGenSizeZ          int    `json:"worldGenSizeZ"`
	StartingZ              int    `json:"startingZ"`
	Title                  string `json:"title"`
	DPI                    int    `json:"dpi"`
	BlueprintPath          string `json:"blueprintPath"`
	RenderPathfindingSteps bool   `json:"renderPathfindingSteps"`
	DebugDisableLighting   bool   `json:"debugDisableLighting"`
	DebugDisableLookdown   bool   `json:"debugDisableLookdown"`
	DebugShowAutotileMask  bool   `json:"debugShowAutotileMask"`
	DebugShowScenarioID    bool   `json:"debugShowScenarioID"`
	ProfileCPU             bool   `json:"profileCPU"`
	ProfileMemory          bool   `json:"profileMemory"`
	OverrideGCMemoryLimit  int64  `json:"overrideGCMemoryLimit"`
	// Rogue mode key-repeat: frames before held WASD starts repeating, and the
	// interval between repeated moves. At 60 fps, 20 delay ≈ 333 ms, 4 interval ≈ 67 ms.
	RogueKeyRepeatDelay    int `json:"rogueKeyRepeatDelay"`
	RogueKeyRepeatInterval int `json:"rogueKeyRepeatInterval"`
	RogueAutoMoveInterval  int `json:"rogueAutoMoveInterval"`
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
