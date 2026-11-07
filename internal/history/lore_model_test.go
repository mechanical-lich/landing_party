package history

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLoreHistoryJSONRoundTrip(t *testing.T) {
	h := New(42)
	h.Civs = map[string]*Civ{"civ1": {ID: "civ1", Name: "Vael", Fate: FateFled, Species: "greyish"}}
	h.Figures = map[string]*Figure{"fig1": {ID: "fig1", Name: "Ossk", Role: "chief_engineer", Civ: "civ1", Fate: "died"}}
	h.Sites = map[string]*SiteHistory{"site1": {ID: "site1", Map: "moon", Arc: "lifeless", State: InitialState(), Events: []string{"ev1"}}}
	h.Events = []Event{{ID: "ev1", Date: -120, Type: "survey", Site: "site1", Civs: []string{"civ1"}, Origin: &SitePoint{X: 3, Y: 4}, Vars: map[string]string{"k": "v"}}}
	h.Known = map[string]bool{"ev1": true}
	h.NextID = 2
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var back History
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h, &back) {
		t.Errorf("round trip changed the history:\n%+v\n%+v", h, &back)
	}
}
