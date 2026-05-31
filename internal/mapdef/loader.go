package mapdef

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

var loaded []MapDef

// Load reads every *.json file in dir as a MapDef. Replaces any previously
// loaded set.
func Load(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("mapdef.Load: %w", err)
	}
	loaded = loaded[:0]
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("mapdef.Load %s: %w", entry.Name(), err)
		}
		var m MapDef
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("mapdef.Load %s: %w", entry.Name(), err)
		}
		if err := m.Size.Validate(); err != nil {
			return fmt.Errorf("mapdef.Load %s: size: %w", entry.Name(), err)
		}
		loaded = append(loaded, m)
	}
	return nil
}

// All returns every loaded map definition.
func All() []MapDef {
	return loaded
}

// ByID returns the map with the given ID, or nil if not found.
func ByID(id string) *MapDef {
	for i := range loaded {
		if loaded[i].ID == id {
			return &loaded[i]
		}
	}
	return nil
}

// Random returns a random loaded map, or nil if none are loaded.
func Random() *MapDef {
	if len(loaded) == 0 {
		return nil
	}
	return &loaded[rand.Intn(len(loaded))]
}
