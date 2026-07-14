// Package audio is landing_party's game-side audio policy: it owns an
// mlge/audio.Mixer, loads sound assets, and maps game/UI events to sounds. The
// reusable engine (buses, pooling, voice management) lives in mlge/audio; this
// package decides *what* plays *when* for this game.
package audio

import (
	"encoding/json"
	"os"
)

// Config is the data-driven sound map (data/audio.json).
//
//	{
//	  "clips": { "click": "assets/audio/ui/click.ogg" },
//	  "ui":    { "ui.button.click": "click" }
//	}
//
// Clips maps a short clip key to an asset path (format inferred from the .ogg /
// .mp3 extension). UI maps a minui event-type string to a clip key. Keeping both
// in data lets each game skin its own sounds without code changes.
type Config struct {
	Clips map[string]string `json:"clips"`
	UI    map[string]string `json:"ui"`
}

// loadConfig reads and parses the sound map. A missing file is not an error —
// it yields an empty config so the game runs silently until sounds are added.
func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
