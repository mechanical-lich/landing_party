package history

import "path/filepath"

// StructurePreset is one entry of data/history/structure_presets.json: a
// structure as history stamps it (script, footprint, params), for the
// structure viewer's one-click presets (lore plan, step 8d).
type StructurePreset struct {
	Group  string         `json:"group"`
	Name   string         `json:"name"`
	Script string         `json:"script"`
	W      int            `json:"w"`
	H      int            `json:"h"`
	Params map[string]any `json:"params"`
}

// LoadPresets reads structure_presets.json from a history data directory.
func LoadPresets(dir string) ([]StructurePreset, error) {
	var f struct {
		Presets []StructurePreset `json:"presets"`
	}
	if err := readJSON(filepath.Join(dir, "structure_presets.json"), &f); err != nil {
		return nil, err
	}
	return f.Presets, nil
}
