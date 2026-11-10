package factory

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

func loadHive(t *testing.T) {
	t.Helper()
	if err := FactoryLoad("../../data/blueprints/entities/hive.json"); err != nil {
		t.Fatal(err)
	}
	old := TypeSource
	TypeSource = func(f, typ, presence string) *UnitType {
		if f != "hive" {
			return nil
		}
		return testTypes[typ]
	}
	t.Cleanup(func() { TypeSource = old })
}

// testTypes mirror data/factions.json's hive types (the content test in
// internal/game checks the file itself).
var testTypes = map[string]*UnitType{
	"drone":    {ID: "drone", Morph: "crawler", SpriteDX: 0, SeenName: "Xeno", HatchTurns: 20, Health: 14, Str: 8, Dex: 12, AC: 6, Attack: "1d4", Initiative: 10},
	"scout":    {ID: "scout", Morph: "flier", SpriteDX: 0, SeenName: "Troid", HatchTurns: 16, Health: 10, Str: 6, Dex: 18, AC: 8, Attack: "1d4", Initiative: 6},
	"warrior":  {ID: "warrior", Morph: "crawler", SpriteDX: 144, SeenName: "Red Xeno", HatchTurns: 24, Health: 32, Str: 16, Dex: 12, AC: 12, Attack: "2d6", Initiative: 12},
	"hunter":   {ID: "hunter", Morph: "flier", SpriteDX: 144, SeenName: "Orange Troid", HatchTurns: 24, Health: 24, Str: 14, Dex: 16, AC: 10, Attack: "1d8", Initiative: 8},
	"burrower": {ID: "burrower", Morph: "crawler", SpriteDX: 288, SeenName: "Purple Xeno", HatchTurns: 30, Health: 24, Str: 14, Dex: 10, AC: 14, Attack: "1d8", Initiative: 14},
	"spitter":  {ID: "spitter", Morph: "flier", SpriteDX: 288, SeenName: "Green Troid", HatchTurns: 30, Health: 18, Str: 10, Dex: 14, AC: 8, Attack: "1d4", Initiative: 10},
}

// Every type shifts body, egg and shell by its colour block, names them, gives
// the egg its hatch time and the adult its absolute stats; a wild creature
// (no type) is the blueprint as it is.
func TestApplyType(t *testing.T) {
	loadHive(t)
	for typ, ut := range testTypes {
		bp := map[string]string{"crawler": "xeno", "flier": "troid"}[ut.Morph]
		baseAdult, _ := Create(bp+"_stage3", 0, 0, 0)
		baseEgg, _ := Create(bp+"_egg", 0, 0, 0)
		ba := baseAdult.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX
		be := baseEgg.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX

		adult, err := CreateTyped(bp+"_stage3", 1, 2, 3, typ, "fp1")
		if err != nil {
			t.Fatal(err)
		}
		a := adult.GetComponent(components.Appearance).(*components.AppearanceComponent)
		h := adult.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		s := adult.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		ic := adult.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		name := adult.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
		if a.SpriteX-ba != ut.SpriteDX || h.MaxHealth != ut.Health || h.Health != ut.Health || s.Str != ut.Str ||
			s.Dex != ut.Dex || s.AC != ut.AC || s.BasicAttackDice != ut.Attack || ic.DefaultValue != ut.Initiative || name != ut.SeenName {
			t.Errorf("%s adult: dx %d hp %d/%d str %d dex %d ac %d %s init %d %q", typ, a.SpriteX-ba, h.Health, h.MaxHealth, s.Str, s.Dex, s.AC, s.BasicAttackDice, ic.DefaultValue, name)
		}
		if k := components.KinOf(adult); k.Type != typ || k.Presence != "fp1" || k.Morph != ut.Morph {
			t.Errorf("%s: kin %+v", typ, k)
		}
		egg, _ := CreateTyped(bp+"_egg", 0, 0, 0, typ, "fp1")
		ea := egg.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX
		ai := egg.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
		ename := egg.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
		if ea-be != ut.SpriteDX || ai.Vars["hatch_turns"] != float64(ut.HatchTurns) || ename != ut.SeenName+" Egg" {
			t.Errorf("%s egg: dx %d hatch %v %q", typ, ea-be, ai.Vars["hatch_turns"], ename)
		}
		if eh := egg.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent); eh.MaxHealth != 8 {
			t.Errorf("%s egg took adult stats: hp %d", typ, eh.MaxHealth)
		}
	}
	// The egg colours the plan names: olive (144,744), red (288,744), pink
	// (432,744); blue (0,768), orange (144,768), green (288,768).
	for typ, x := range map[string]int{"drone": 144, "warrior": 288, "burrower": 432, "scout": 0, "hunter": 144, "spitter": 288} {
		bp := map[string]string{"crawler": "xeno", "flier": "troid"}[testTypes[typ].Morph]
		e, _ := CreateTyped(bp+"_egg", 0, 0, 0, typ, "")
		if a := e.GetComponent(components.Appearance).(*components.AppearanceComponent); a.SpriteX != x {
			t.Errorf("%s egg at x %d, want %d", typ, a.SpriteX, x)
		}
	}
	wild, _ := CreateTyped("xeno_stage3", 0, 0, 0, "", "")
	base, _ := Create("xeno_stage3", 0, 0, 0)
	if wild.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth != base.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth ||
		wild.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX != base.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX {
		t.Error("a wild creature changed")
	}
}

// spawn_kin's helper names the next stage after the parent's own morph and
// carries its type and presence, so a red egg hatches a red hatchling of the
// same hive.
func TestSpawnKinCarriesTypeAndPresence(t *testing.T) {
	loadHive(t)
	egg, _ := CreateTyped("troid_egg", 0, 0, 0, "hunter", "fp7")
	h := SpawnKin(egg, "hatchling", 4, 5, 6)
	if h == nil || h.Blueprint != "troid_hatchling" {
		t.Fatalf("hatchling = %+v", h)
	}
	k := components.KinOf(h)
	if k.Type != "hunter" || k.Presence != "fp7" || k.Morph != "flier" {
		t.Errorf("kin = %+v", k)
	}
	if a := h.GetComponent(components.Appearance).(*components.AppearanceComponent); a.SpriteX != 144 {
		t.Errorf("sprite x = %d, want the orange block", a.SpriteX)
	}
	if SpawnKin(egg, "stage9", 0, 0, 0) != nil {
		t.Error("a stage with no blueprint spawned something")
	}
}

// A Phase B save's tier maps to the type of that colour.
func TestKinLegacyTier(t *testing.T) {
	for _, c := range []struct {
		js, want string
	}{
		{`{"Morph":"crawler","tier":1,"presence":"fp1"}`, "warrior"},
		{`{"Morph":"crawler","tier":2}`, "burrower"},
		{`{"Morph":"flier","tier":1}`, "hunter"},
		{`{"Morph":"flier","tier":2}`, "spitter"},
		{`{"Morph":"flier"}`, ""},
		{`{"morph":"crawler","type":"drone"}`, "drone"},
	} {
		var k components.KinComponent
		if err := json.Unmarshal([]byte(c.js), &k); err != nil || k.Type != c.want {
			t.Errorf("%s: %+v %v, want %s", c.js, k, err, c.want)
		}
	}
}

// Both morphs are one faction (never fight each other) and fliers fly.
func TestHiveMorphsShareAFaction(t *testing.T) {
	loadHive(t)
	for _, bp := range []string{"xeno_egg", "xeno_hatchling", "xeno_stage2", "xeno_stage3", "troid_egg", "troid_hatchling", "troid_stage2", "troid_stage3", "egg_launcher"} {
		e, err := Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "hive" {
			t.Errorf("%s faction %q", bp, f)
		}
		flier := components.KinOf(e).Morph == "flier"
		flies := false
		if e.HasComponent(components.Skills) {
			for _, s := range e.GetComponent(components.Skills).(*components.SkillsComponent).Skills {
				flies = flies || s == "flying"
			}
		}
		if flier && bp != "troid_egg" && !flies {
			t.Errorf("%s is a flier that cannot fly", bp)
		}
		if !flier && flies {
			t.Errorf("%s flies", bp)
		}
	}
}
