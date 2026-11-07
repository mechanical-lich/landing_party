package history

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// A tech role reads as one structure or a list of alternatives; each stamp
// picks one by weight, the same one for the same key.
func TestContentTechRolePicks(t *testing.T) {
	var roles map[string]TechRole
	err := json.Unmarshal([]byte(`{
		"name": "castle",
		"one": {"script": "fort", "w": 21, "h": 17},
		"even": [{"script": "castle_town"}, {"script": "castle"}],
		"heavy": [{"script": "a", "weight": 3}, {"script": "b"}]
	}`), &roles)
	if err != nil {
		t.Fatal(err)
	}
	if r := roles["name"]; len(r) != 1 || r[0].Script != "castle" {
		t.Errorf("bare name read as %+v", r)
	}
	if r := roles["one"]; len(r) != 1 || r[0].W != 21 {
		t.Errorf("object read as %+v", r)
	}
	share := func(role, script string) float64 {
		n := 0
		for i := 0; i < 4000; i++ {
			key := fmt.Sprintf("%d/ev%d", i*7919, i%40)
			if roles[role].Pick(key).Script == script {
				n++
			}
			if roles[role].Pick(key).Script != roles[role].Pick(key).Script {
				t.Fatalf("%s: key %s picks differently", role, key)
			}
		}
		return float64(n) / 4000
	}
	if s := share("even", "castle_town"); math.Abs(s-0.5) > 0.05 {
		t.Errorf("even: castle_town picked %.0f%%, want about 50%%", s*100)
	}
	if s := share("heavy", "a"); math.Abs(s-0.75) > 0.05 {
		t.Errorf("heavy: a picked %.0f%%, want about 75%%", s*100)
	}
	if (TechRole{}).Pick("x").Script != "" {
		t.Error("an empty role picks a structure")
	}
}
