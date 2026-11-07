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
	maps, err := ReadDir(dir)
	if err != nil {
		return err
	}
	loaded = append(loaded[:0], maps...)
	return nil
}

// ReadDir reads every map definition in dir without touching the loaded set,
// for callers (the history library) that need the maps before or apart from
// the game's own load.
func ReadDir(dir string) ([]MapDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("mapdef.Load: %w", err)
	}
	var out []MapDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("mapdef.Load %s: %w", entry.Name(), err)
		}
		var m MapDef
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("mapdef.Load %s: %w", entry.Name(), err)
		}
		if err := m.Size.Validate(); err != nil {
			return nil, fmt.Errorf("mapdef.Load %s: size: %w", entry.Name(), err)
		}
		out = append(out, m)
	}
	return out, nil
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

// Random returns a random loaded map, or nil if none are loaded. Internal maps
// are skipped.
func Random() *MapDef {
	var pool []*MapDef
	for i := range loaded {
		if !loaded[i].Internal {
			pool = append(pool, &loaded[i])
		}
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[rand.Intn(len(pool))]
}
