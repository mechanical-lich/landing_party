package crafting

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
)

type Recipe struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Output      string         `json:"output"`
	BuildTime   int            `json:"build_time"`
	Cost        map[string]int `json:"cost"`
	Station     string         `json:"station"`
}

var recipes map[string]Recipe

func init() {
	recipes = make(map[string]Recipe)
	file, err := os.OpenFile("data/crafting_recipes.json", os.O_RDONLY, 0644)
	if err != nil {
		log.Print("Failed to open crafting_recipes.json:", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		log.Print("Failed to read crafting_recipes.json:", err)
		return
	}
	if err := json.Unmarshal(data, &recipes); err != nil {
		log.Print("Failed to unmarshal crafting_recipes.json:", err)
	}
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
