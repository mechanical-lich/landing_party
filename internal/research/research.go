package research

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

type Tech struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Duration         int      `json:"duration"`
	RequiredBuilding string   `json:"required_building"`
	RequiredInt      int      `json:"required_int"`
	RequiresTech     string   `json:"requires_tech"`
	Unlocks          []string `json:"unlocks"`
}

var techs map[string]Tech

func init() {
	techs = make(map[string]Tech)
	file, err := os.OpenFile("data/research.json", os.O_RDONLY, 0644)
	if err != nil {
		log.Print("Failed to open research.json:", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		log.Print("Failed to read research.json:", err)
		return
	}
	if err := json.Unmarshal(data, &techs); err != nil {
		log.Print("Failed to unmarshal research.json:", err)
	}
}

func GetTech(key string) (Tech, bool) {
	t, ok := techs[key]
	return t, ok
}

func AllTechs() map[string]Tech {
	return techs
}

// AvailableTechs returns techs that can be researched given the current known techs.
func AvailableTechs(knownTechs []string) []Tech {
	known := make(map[string]bool, len(knownTechs))
	for _, k := range knownTechs {
		known[k] = true
	}
	var result []Tech
	for _, t := range techs {
		if known[t.Name] {
			continue
		}
		if t.RequiresTech != "" && !known[t.RequiresTech] {
			continue
		}
		result = append(result, t)
	}
	return result
}
