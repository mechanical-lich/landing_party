package crafting

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
)

type Recipe struct {
	// Key is the recipe's map key in crafting_recipes.json, stamped on load.
	Key          string         `json:"-"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Output       string         `json:"output"`
	BuildTime    int            `json:"build_time"`
	Cost         map[string]int `json:"cost"`
	Station      string         `json:"station"`
	RequiresTech string         `json:"requires_tech,omitempty"`
	// DeployOnCraft puts the output on the level at the station as a worker
	// of the crafter's settlement (a printed colonist) instead of into the
	// crafter's bag (finding 4 of the enemy factions plan). A director
	// printing a unit at its own fabricator sets it too.
	DeployOnCraft bool `json:"deploy_on_craft,omitempty"`
	// KinType is the unit type the output is given (factory.ApplyType; the
	// hive's eggs, coloured by what will hatch), with the crafter's presence
	// and settlement. ConsumeStation: the station is used up, and a deployed
	// output stands on its tile in its place (an egg laid on a grey egg).
	// hive_plan.md, "Larvae and eggs".
	KinType        string `json:"kin_type,omitempty"`
	ConsumeStation bool   `json:"consume_station,omitempty"`
	// Count is how many of a material output one craft makes (the autonz'
	// refine_ferrite: two); 0 or 1 is one.
	Count int `json:"count,omitempty"`
}

var recipes map[string]Recipe

func init() {
	if err := Load("data/crafting_recipes.json"); err != nil {
		log.Print("Failed to load crafting_recipes.json:", err)
	}
}

// Load replaces the recipe table with path's (empty when it cannot be read;
// Load("") clears it). init loads data/crafting_recipes.json relative to the
// cwd.
func Load(path string) error {
	recipes = make(map[string]Recipe)
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &recipes); err != nil {
		return err
	}
	for k, r := range recipes {
		r.Key = k
		recipes[k] = r
	}
	return nil
}

func GetRecipe(key string) (Recipe, bool) {
	r, ok := recipes[key]
	return r, ok
}

func AllRecipes() []Recipe {
	result := make([]Recipe, 0, len(recipes))
	for _, r := range recipes {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func RecipesByStation(stationID string) []Recipe {
	result := make([]Recipe, 0)
	for _, r := range recipes {
		if r.Station == stationID {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
