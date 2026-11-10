package factory

import (
	"strings"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// Unit types: a faction's specialisations applied at spawn, so one blueprint
// per morph and life stage serves every type (docs/brainstorm/hive_plan.md,
// "Types in data/factions.json"). Each colour block on the sprite sheet is a
// fixed offset from the previous; a type adds its block's offset to the body
// and the egg alike, and gives the adult its stats, role and kit. The table
// comes from the faction's doctrine (data/factions.json "types"), with the
// presence's known upgrades folded in, supplied by TypeSource so this
// package needs no campaign import.

// UnitType is one unit type as a body spawns with it: the doctrine's type with
// its known upgrades applied.
type UnitType struct {
	ID       string
	Morph    string
	SpriteDX int
	// SpriteDY and Tier are the print tier's (the autonz' bronze is one
	// row down), recorded on the Kin component at spawn.
	SpriteDY int
	Tier     string
	// SeenName is the body's name ("Red Xeno"; its egg "Red Xeno Egg").
	SeenName   string
	HatchTurns int
	Health     int
	Str        int
	Dex        int
	AC         int
	Attack     string
	Initiative int
	Role       string
	// Tasks are the worker's allowed task actions; nil leaves the worker's
	// own (a non-nil empty list: it picks no task, it is only sent).
	Tasks  []task.TaskAction
	Skills []string
	// BodySkills are added at every life stage (eggs and hatchlings too).
	BodySkills []string
	Equip      []string
	// Evolved draws a working adult in the morph's evolved (stage 3) frame:
	// its type's second upgrade is known.
	Evolved bool
	// Work multiplies a worker's work speed (an upgrade).
	Work float64
}

// TypeSource returns a faction's unit type typ for a body of presence (whose
// known upgrades apply), or nil when the faction has no such type. The game
// wires it to the doctrine file; tests set it directly.
var TypeSource func(faction, typ, presence string) *UnitType

// TypeOf is faction's type typ for presence, nil without one.
func TypeOf(faction, typ, presence string) *UnitType {
	if TypeSource == nil || faction == "" || typ == "" {
		return nil
	}
	return TypeSource(faction, typ, presence)
}

// TaskActions maps a type's task names to task actions ("chop" is the mine
// order on a choppable, "haul" the retrieve order).
func TaskActions(names []string) []task.TaskAction {
	if names == nil {
		return nil
	}
	out := []task.TaskAction{}
	seen := map[task.TaskAction]bool{}
	for _, n := range names {
		a := task.TaskAction(n)
		switch n {
		case "chop":
			a = task_requests.MineAction
		case "haul":
			a = task_requests.RetrieveAction
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// CreateTyped creates a blueprint and applies typ for presence (see
// ApplyType).
func CreateTyped(name string, x, y, z int, typ, presence string) (*ecs.Entity, error) {
	e, err := Create(name, x, y, z)
	if err != nil {
		return nil, err
	}
	ApplyType(e, typ, presence)
	return e, nil
}

// lifeStage is a hive blueprint's stage by its suffix: "egg", "cracked",
// "hatchling" or "adult".
func lifeStage(blueprint string) string {
	switch {
	case strings.HasSuffix(blueprint, "_egg_cracked"):
		return "cracked"
	case strings.HasSuffix(blueprint, "_egg"):
		return "egg"
	case strings.HasSuffix(blueprint, "_hatchling"):
		return "hatchling"
	}
	return "adult"
}

// ApplyType gives a freshly created entity its faction's unit type: the
// type's colour block added to its sprite (body, egg and shell alike), its
// name, an egg's hatch time, and an adult's absolute stats, initiative,
// skills, role and allowed tasks, and the evolved frame for a working adult
// whose type's second upgrade is known. The type and presence are recorded
// on its Kin component (added if the blueprint has none). "" records only
// the presence (a wild creature); a faction with no such type only records
// it. Call it on a fresh entity only: it is not idempotent.
func ApplyType(e *ecs.Entity, typ, presence string) {
	if e == nil {
		return
	}
	faction := ""
	if e.HasComponent(rlcomponents.Description) {
		faction = e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	}
	kin := components.KinOf(e)
	if kin == nil {
		kin = &components.KinComponent{}
		e.AddComponent(kin)
	}
	kin.Type, kin.Presence = typ, presence
	t := TypeOf(faction, typ, presence)
	if t == nil {
		return
	}
	stage := lifeStage(e.Blueprint)
	kin.Tier = t.Tier
	if e.HasComponent(components.Appearance) {
		a := e.GetComponent(components.Appearance).(*components.AppearanceComponent)
		a.SpriteX += t.SpriteDX
		a.SpriteY += t.SpriteDY
		if t.Evolved && stage == "adult" && e.HasComponent(components.Worker) {
			a.SpriteX += 48 // the stage-3 frame beside the stage-2 one
		}
	}
	if e.HasComponent(rlcomponents.Description) && t.SeenName != "" {
		dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		switch stage {
		case "egg":
			dc.Name = t.SeenName + " Egg"
		case "hatchling":
			dc.Name = t.SeenName + " Hatchling"
		case "adult":
			dc.Name = t.SeenName
		}
	}
	addSkills(e, t.BodySkills)
	switch stage {
	case "egg":
		if t.HatchTurns > 0 && e.HasComponent(components.ScriptedAI) {
			sc := e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
			if sc.Vars == nil {
				sc.Vars = map[string]any{}
			}
			sc.Vars["hatch_turns"] = float64(t.HatchTurns)
		}
		return
	case "adult":
	default:
		return
	}
	if e.HasComponent(rlcomponents.Health) && t.Health > 0 {
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		hc.MaxHealth, hc.Health = t.Health, t.Health
	}
	if e.HasComponent(rlcomponents.Stats) {
		sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		if t.Str > 0 {
			sc.Str = t.Str
		}
		if t.Dex > 0 {
			sc.Dex = t.Dex
		}
		if t.AC > 0 {
			sc.AC = t.AC
		}
		if t.Attack != "" {
			sc.BasicAttackDice = t.Attack
		}
	}
	if e.HasComponent(rlcomponents.Initiative) && t.Initiative > 0 {
		ic := e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		ic.DefaultValue = t.Initiative
	}
	addSkills(e, t.Skills)
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if t.Role != "" {
			wc.Role = t.Role
		}
		if t.Work > 0 {
			wc.WorkMult = t.Work
		}
		if t.Tasks != nil {
			wc.AvailableTasks = append([]task.TaskAction{}, t.Tasks...)
			wc.AllowedTasks = append([]task.TaskAction{}, t.Tasks...)
		}
	}
	// The type's kit (the spitter's gland, a natural weapon): into the bag
	// and equipped. A body's bag is never dropped when it dies, so a natural
	// weapon is never looted.
	if len(t.Equip) > 0 && e.HasComponent(rlcomponents.Inventory) {
		inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, bp := range t.Equip {
			if it, err := Create(bp, 0, 0, 0); err == nil {
				inv.AddItem(it)
			}
		}
		inv.EquipAllBest()
	}
}

// addSkills adds skills e lacks (a Skills component if it has none).
func addSkills(e *ecs.Entity, skills []string) {
	if len(skills) == 0 {
		return
	}
	sk, _ := e.GetComponent(components.Skills).(*components.SkillsComponent)
	if sk == nil {
		sk = &components.SkillsComponent{}
		e.AddComponent(sk)
	}
	for _, s := range skills {
		if !sk.Has(s) {
			sk.Skills = append(sk.Skills, s)
		}
	}
}

// Retype re-applies a living adult's type with its presence's upgrades as
// they stand now (an evolve entry completed): its sprite block and frame,
// absolute stats, initiative and work speed, its current health kept in
// proportion. Its kit and name are left as they are. Eggs and hatchlings
// are untouched: they get the upgrades when they grow.
func Retype(e *ecs.Entity) {
	k := components.KinOf(e)
	if k == nil || k.Type == "" || lifeStage(e.Blueprint) != "adult" {
		return
	}
	faction := ""
	if e.HasComponent(rlcomponents.Description) {
		faction = e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	}
	t := TypeOf(faction, k.Type, k.Presence)
	if t == nil || t.Tier != k.Tier {
		return // a print tier is at spawn only (a steel autonz stays steel)
	}
	if e.HasComponent(components.Appearance) {
		if base, err := Create(e.Blueprint, 0, 0, 0); err == nil && base.HasComponent(components.Appearance) {
			a := e.GetComponent(components.Appearance).(*components.AppearanceComponent)
			a.SpriteX = base.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX + t.SpriteDX
			if t.Evolved && e.HasComponent(components.Worker) {
				a.SpriteX += 48
			}
		}
	}
	if e.HasComponent(rlcomponents.Health) && t.Health > 0 {
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		if hc.MaxHealth > 0 {
			hc.Health = max(1, hc.Health*t.Health/hc.MaxHealth)
		}
		hc.MaxHealth = t.Health
	}
	if e.HasComponent(rlcomponents.Stats) && t.Str > 0 {
		e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).Str = t.Str
	}
	if e.HasComponent(rlcomponents.Initiative) && t.Initiative > 0 {
		e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent).DefaultValue = t.Initiative
	}
	if e.HasComponent(components.Worker) && t.Work > 0 {
		e.GetComponent(components.Worker).(*components.WorkerComponent).WorkMult = t.Work
	}
}

// KinBlueprint is the blueprint of the life stage stage of the same morph as
// blueprint: the morph prefix is everything before the first '_' ("xeno_egg",
// "hatchling" → "xeno_hatchling"). "" when blueprint has no prefix.
func KinBlueprint(blueprint, stage string) string {
	i := strings.IndexByte(blueprint, '_')
	if i <= 0 {
		return ""
	}
	return blueprint[:i] + "_" + stage
}

// KinStage is the blueprint spawn_kin makes for stage of parent's morph:
// KinStageOverride may redirect it (a settled presence's adult is the morph's
// crew body; the game wires it), else KinBlueprint.
func KinStage(parent *ecs.Entity, stage string) string {
	if KinStageOverride != nil {
		if bp := KinStageOverride(parent, stage); bp != "" {
			return bp
		}
	}
	return KinBlueprint(parent.Blueprint, stage)
}

// KinStageOverride, when set, names the blueprint for parent's next stage, ""
// for the default.
var KinStageOverride func(parent *ecs.Entity, stage string) string

// KinPrepare, when set, is called on every body spawn_kin makes before its
// type is applied (the game gives a settled presence's hatchling its nursery
// script and enrols its adult as a worker of the settlement, so the type's
// role and tasks land on it).
var KinPrepare func(parent, child *ecs.Entity, stage string)

// SpawnKin creates the life stage stage of parent's morph at (x, y, z) with
// parent's type, presence and settlement. nil when there is no such
// blueprint.
func SpawnKin(parent *ecs.Entity, stage string, x, y, z int) *ecs.Entity {
	if parent == nil {
		return nil
	}
	bp := KinStage(parent, stage)
	if bp == "" || !BlueprintExists(bp) {
		return nil
	}
	typ, presence, settled := "", "", ""
	if k := components.KinOf(parent); k != nil {
		typ, presence, settled = k.Type, k.Presence, k.Settlement
	}
	e, err := Create(bp, x, y, z)
	if err != nil {
		return nil
	}
	if k := components.KinOf(e); k != nil {
		k.Settlement = settled
	} else {
		e.AddComponent(&components.KinComponent{Settlement: settled})
	}
	if KinPrepare != nil {
		KinPrepare(parent, e, stage)
	}
	ApplyType(e, typ, presence)
	return e
}
