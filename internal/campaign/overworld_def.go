package campaign

import (
	"encoding/json"
	"fmt"
	"os"
)

type overworldFile struct {
	Locations []Location `json:"locations"`
}

// LoadOverworldDefs reads the data-driven overworld definition (the set of
// locations and their generation recipes). Mirrors mapdef/scenario loaders.
func LoadOverworldDefs(path string) ([]Location, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("campaign.LoadOverworldDefs: %w", err)
	}
	var f overworldFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("campaign.LoadOverworldDefs: %w", err)
	}
	return f.Locations, nil
}
