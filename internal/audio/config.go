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
// ClipPaths is one or more asset paths behind a clip key. It accepts either a
// single JSON string ("a.ogg") or an array (["a_0.ogg","a_1.ogg"]); an array
// registers a variation set the mixer picks from at random.
type ClipPaths []string

func (c *ClipPaths) UnmarshalJSON(b []byte) error {
	// Try a single string first, then fall back to a string array.
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*c = ClipPaths{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*c = many
	return nil
}

// Clips maps a clip key to one or more asset paths (format inferred from the
// .ogg / .mp3 extension; multiple paths become a random-variation set). UI maps
// a minui event-type string to a clip key. World maps a world.SoundTag string
// (e.g. "gunshot", "impact") to a clip key for positional in-world SFX. Keeping
// these in data lets each game skin its own sounds without code changes.
type Config struct {
	Clips map[string]ClipPaths `json:"clips"`
	UI    map[string]string    `json:"ui"`
	World map[string]string    `json:"world"`
	// Music maps a music state ("menu", "field", "combat") to its track files;
	// the director rotates randomly among a state's tracks.
	Music map[string][]string `json:"music"`
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
