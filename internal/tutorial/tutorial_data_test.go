package tutorial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

const dataRoot = "../../data"

// jsonKeys returns the top-level keys of a JSON object file.
func jsonKeys(t *testing.T, path string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// blueprintIDs collects every blueprint key under data/blueprints.
func blueprintIDs(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(dataRoot, "blueprints"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		for k := range jsonKeys(t, path) {
			out[k] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The real step file loads, validates, and refers only to things that exist.
func TestRealDefinition(t *testing.T) {
	def, err := Load(filepath.Join(dataRoot, "tutorial.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Steps) != 24 {
		t.Errorf("%d steps, want 24", len(def.Steps))
	}
	if def.Steps[0].ID != "choose_planet" || def.Steps[len(def.Steps)-1].ID != "first_build" {
		t.Errorf("steps run %s .. %s", def.Steps[0].ID, def.Steps[len(def.Steps)-1].ID)
	}

	blueprints := blueprintIDs(t)
	builds := jsonKeys(t, filepath.Join(dataRoot, "build.json"))
	techs := jsonKeys(t, filepath.Join(dataRoot, "research.json"))

	unlocks := 0
	for _, s := range def.Steps {
		if s.UnlockNext {
			unlocks++
		}
		if s.Title == "" || s.Text == "" {
			t.Errorf("step %s has no title or text", s.ID)
		}
		switch s.Done.Trigger {
		case objective.TriggerStructureBuilt:
			if !builds[s.Done.Structure] {
				t.Errorf("step %s: structure %q is not buildable", s.ID, s.Done.Structure)
			}
		case objective.TriggerTechResearched:
			if !techs[s.Done.TechKey] {
				t.Errorf("step %s: tech %q does not exist", s.ID, s.Done.TechKey)
			}
		case objective.TriggerResourceGathered:
			if s.Done.Resource != "food" && !blueprints[s.Done.Resource] {
				t.Errorf("step %s: resource %q is not a blueprint", s.ID, s.Done.Resource)
			}
			if s.Done.Op == "" {
				t.Errorf("step %s: resource_gathered without op", s.ID)
			}
		case objective.TriggerFlagSet:
			if !strings.HasPrefix(s.Done.Flag, "ui.") {
				t.Errorf("step %s: flag %q is not a ui.* flag", s.ID, s.Done.Flag)
			}
			if bp, ok := strings.CutPrefix(s.Done.Flag, "ui.carrying:"); ok && !blueprints[bp] {
				t.Errorf("step %s: carried blueprint %q does not exist", s.ID, bp)
			}
		case objective.TriggerSettlementPopulation:
			if s.Done.Settlement != "colony" || s.Done.Op == "" {
				t.Errorf("step %s: population rule %+v", s.ID, s.Done)
			}
		default:
			t.Errorf("step %s: unexpected trigger %q", s.ID, s.Done.Trigger)
		}
		for _, h := range s.Highlight {
			kind, name, ok := strings.Cut(h, ":")
			if !ok {
				t.Errorf("step %s: highlight %q has no kind", s.ID, h)
				continue
			}
			switch kind {
			case "hud":
				if id, ok := strings.CutPrefix(name, "build_"); ok && !builds[id] {
					t.Errorf("step %s: highlight %q names an unknown buildable", s.ID, h)
				}
			case "entity":
				if !blueprints[name] {
					t.Errorf("step %s: highlight %q names an unknown blueprint", s.ID, h)
				}
			case "star":
				if name != AtTutorial && name != AtNext {
					t.Errorf("step %s: highlight %q", s.ID, h)
				}
			case "ship":
				if name != "hold" {
					t.Errorf("step %s: highlight %q", s.ID, h)
				}
			case "dash", "region", "inspector":
			default:
				t.Errorf("step %s: unknown highlight kind %q", s.ID, h)
			}
		}
		if s.Target != "" && !strings.Contains(s.Context, "starmap") {
			t.Errorf("step %s: target set off the Star Map", s.ID)
		}
	}
	if unlocks != 1 {
		t.Errorf("%d unlock_next steps, want exactly 1", unlocks)
	}
	if len(def.Topics) == 0 {
		t.Fatal("no Field Manual topics")
	}
	seen := map[string]bool{}
	for _, tp := range def.Topics {
		if tp.ID == "" || tp.Title == "" || tp.Body == "" {
			t.Errorf("topic %+v incomplete", tp)
		}
		if seen[tp.ID] {
			t.Errorf("duplicate topic %q", tp.ID)
		}
		seen[tp.ID] = true
	}
}
