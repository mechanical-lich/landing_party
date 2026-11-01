package config

import (
	"encoding/json"
	"testing"
)

// The tutorial default is layered onto config.local.json without touching
// other overrides, and the field reads back into Config.
func TestMergeTutorialOverridePreservesExisting(t *testing.T) {
	existing := []byte(`{"uiVolume": 0.5, "unlockAllResearch": true}`)
	out, err := mergeTutorialOverride(existing, true)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if got["uiVolume"] != 0.5 || got["unlockAllResearch"] != true {
		t.Fatalf("other overrides lost: %v", got)
	}
	if got["tutorialOffDefault"] != true {
		t.Fatalf("tutorialOffDefault = %v, want true", got["tutorialOffDefault"])
	}

	out, err = mergeTutorialOverride(out, false)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.TutorialOffDefault || cfg.UIVolume != 0.5 {
		t.Fatalf("round trip: off=%v ui=%v", cfg.TutorialOffDefault, cfg.UIVolume)
	}

	out, err = mergeTutorialOverride(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["tutorialOffDefault"] != true {
		t.Fatalf("from empty: %v", got)
	}
}
