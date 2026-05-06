package scenario

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

func SelectRandom() error {
	if len(enabled) == 0 {
		return fmt.Errorf("scenario.SelectRandom: no enabled scenarios")
	}
	s := enabled[rand.Intn(len(enabled))]
	active = &s
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
