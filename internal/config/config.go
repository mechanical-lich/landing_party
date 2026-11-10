package config

import (
	"encoding/json"
	"log"
	"os"
	"testing"
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
	UnlockAllResearch     bool  `json:"unlockAllResearch"`
	ProfileCPU            bool  `json:"profileCPU"`
	ProfileMemory         bool  `json:"profileMemory"`
	OverrideGCMemoryLimit int64 `json:"overrideGCMemoryLimit"`
	// Rogue mode key-repeat: frames before held WASD starts repeating, and the
	// interval between repeated moves. At 60 fps, 20 delay ≈ 333 ms, 4 interval ≈ 67 ms.
	RogueKeyRepeatDelay    int `json:"rogueKeyRepeatDelay"`
	RogueKeyRepeatInterval int `json:"rogueKeyRepeatInterval"`
	RogueAutoMoveInterval  int `json:"rogueAutoMoveInterval"`

	// MaxLoadedPlanets caps the planets kept in memory, not counting the ship. The
	// least recently visited staffed one past the cap freezes to disk.
	MaxLoadedPlanets int `json:"maxLoadedPlanets"`

	// BackgroundTicksPerFrame is how many ticks away planets share each frame, and
	// BackgroundOwedCap how far behind one can fall before excess ticks are dropped.
	BackgroundTicksPerFrame int `json:"backgroundTicksPerFrame"`
	BackgroundOwedCap       int `json:"backgroundOwedCap"`

	// IncidentCooldownTicks is how long a planet's colonists must go unhurt before
	// more damage raises a new alert. NotificationMergeTicks is how far apart repeated
	// notifications still merge. Both in the away planet's own ticks.
	IncidentCooldownTicks  int `json:"incidentCooldownTicks"`
	NotificationMergeTicks int `json:"notificationMergeTicks"`

	// Audio bus volumes, 0..1. Base defaults live in data/config.json; the
	// settings screen writes player overrides to config.local.json.
	UIVolume    float64 `json:"uiVolume"`
	GameVolume  float64 `json:"gameVolume"`
	MusicVolume float64 `json:"musicVolume"`

	// Footstep audio toggles. When off, footsteps of that side are silent but
	// still emit their sound event (AI hearing is unaffected).
	FriendlyFootsteps bool `json:"friendlyFootsteps"`
	EnemyFootsteps    bool `json:"enemyFootsteps"`

	// TutorialOffDefault turns the New Expedition screen's "Guided first
	// landing" toggle off by default. Written to config.local.json by the
	// tutorial's "don't show again" choice; the player can still flip the
	// toggle each time.
	TutorialOffDefault bool `json:"tutorialOffDefault"`

	// ObserverMode makes the game play itself (phase C1 of the enemy
	// factions plan): the director runs the player's settlement, the cursor
	// gives no orders, incident popups are answered Ignore and the Star Map
	// never stops the clock. For integration and balance runs.
	ObserverMode bool `json:"observerMode"`
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
//
// Under go test (testing.Testing()) the file is skipped: it is the player's
// own and git-ignored, so a developer's observer-mode or unlock flags must
// never change what a test sees. The game binary always applies it.
func applyLocalOverrides(cfg *Config) {
	if localOverridesDisabled() {
		return
	}
	applyLocalOverridesFrom(cfg, localConfigPath)
}

// localOverridesDisabled reports whether config.local.json is ignored: always
// under go test, never in the game.
func localOverridesDisabled() bool { return testing.Testing() }

// applyLocalOverridesFrom merges the JSON file at path onto cfg (missing file:
// no-op). Split out so a test can prove the merge itself still works.
func applyLocalOverridesFrom(cfg *Config, localConfigPath string) {
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

// mergeTutorialOverride layers tutorialOffDefault onto the existing
// config.local.json contents, keeping every other local setting. Pure, like
// mergeAudioOverrides.
func mergeTutorialOverride(existing []byte, off bool) ([]byte, error) {
	overrides := map[string]any{}
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &overrides) // best effort; a bad file is replaced
	}
	overrides["tutorialOffDefault"] = off
	return json.MarshalIndent(overrides, "", "  ")
}

// SaveTutorialDefault records whether the New Expedition screen should offer
// the guided first landing by default (off = true turns the toggle off). It
// writes config.local.json, preserving other overrides, and reloads the
// global config. It belongs in config rather than the campaign because it
// outlives any one expedition.
func SaveTutorialDefault(off bool) error {
	existing, _ := os.ReadFile(localConfigPath) // missing file → empty, fine
	out, err := mergeTutorialOverride(existing, off)
	if err != nil {
		return err
	}
	if err := os.WriteFile(localConfigPath, out, 0o644); err != nil {
		return err
	}
	Reload()
	return nil
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
