package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var (
	active  *Scenario
	enabled []Scenario
)

func Load(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("scenario.Load: %w", err)
	}
	enabled = enabled[:0]
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("scenario.Load %s: %w", entry.Name(), err)
		}
		var s Scenario
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("scenario.Load %s: %w", entry.Name(), err)
		}
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	return nil
}

func SelectByID(id string) error {
	for i := range enabled {
		if enabled[i].ID == id {
			active = &enabled[i]
			return nil
		}
	}
	return fmt.Errorf("scenario.SelectByID: %q not found", id)
}

func Active() *Scenario {
	if active == nil {
		panic("scenario.Active called before scenario.Load")
	}
	return active
}

func AllEnabled() []Scenario {
	return enabled
}

// ByID returns the enabled scenario with the given ID, or nil if not found.
// Unlike SelectByID this does not change the active scenario.
func ByID(id string) *Scenario {
	for i := range enabled {
		if enabled[i].ID == id {
			return &enabled[i]
		}
	}
	return nil
}

// ForMap returns the enabled scenarios that support the given map ID.
func ForMap(mapID string) []Scenario {
	var out []Scenario
	for i := range enabled {
		if enabled[i].SupportsMap(mapID) {
			out = append(out, enabled[i])
		}
	}
	return out
}
