package config

import (
	"encoding/json"
	"testing"
)

func TestMergeVolumeOverridesPreservesExisting(t *testing.T) {
	existing := []byte(`{"screenWidth": 1000, "uiVolume": 1.0}`)
	out, err := mergeVolumeOverrides(existing, 0.5, 0.25, 0)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	// Unrelated local override is preserved.
	if got["screenWidth"] != float64(1000) {
		t.Fatalf("screenWidth = %v, want 1000 (preserved)", got["screenWidth"])
	}
	// Volumes are overwritten with the new values (including 0 = muted).
	if got["uiVolume"] != 0.5 || got["gameVolume"] != 0.25 || got["musicVolume"] != float64(0) {
		t.Fatalf("volumes = %v/%v/%v, want 0.5/0.25/0",
			got["uiVolume"], got["gameVolume"], got["musicVolume"])
	}
}

func TestMergeVolumeOverridesFromEmpty(t *testing.T) {
	out, err := mergeVolumeOverrides(nil, 0.8, 0.8, 0.8)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if len(got) != 3 || got["uiVolume"] != 0.8 {
		t.Fatalf("unexpected merged result: %v", got)
	}
}
