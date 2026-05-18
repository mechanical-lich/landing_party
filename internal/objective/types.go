// Package objective is the data-driven condition engine: a Rule with a
// TriggerType is evaluated against an EvalContext. It powers campaign quests.
// It has no notion of "winning" or "losing" — that framing was removed along
// with the old scenario win-condition concept.
package objective

type TriggerType string

const (
	TriggerDaysSurvived         TriggerType = "days_survived"
	TriggerStructureBuilt       TriggerType = "structure_built"
	TriggerTechResearched       TriggerType = "tech_researched"
	TriggerSettlementPopulation TriggerType = "settlement_population"
	TriggerEntityEliminated     TriggerType = "entity_eliminated"
	TriggerColonistEliminated   TriggerType = "colonist_eliminated"
	// TriggerResourceGathered fires when the colony's stored count of Resource
	// satisfies Op/Threshold (e.g. gather 200 metal_ore).
	TriggerResourceGathered TriggerType = "resource_gathered"
	// TriggerEntityKilled fires when the live count of Blueprint satisfies
	// Op/Threshold. "Kill all of X" is op "lte", threshold 0.
	TriggerEntityKilled TriggerType = "entity_killed"
	// TriggerTargetKilled fires once the specific quest target identified by
	// Rule.Target has been killed (a named boss / bounty creature). Unlike
	// entity_killed it tracks one tagged entity, not a blueprint count.
	TriggerTargetKilled TriggerType = "target_killed"
)

type Condition struct {
	GameFlag    *string      `json:"game_flag,omitempty"`
	EntityCount *EntityCount `json:"entity_count,omitempty"`
}

type EntityCount struct {
	Blueprint string `json:"blueprint"`
	Op        string `json:"op"`
	Value     int    `json:"value"`
}

// Rule is a single objective condition. (The former Result/Outcome/Message
// win-condition fields were removed; quests carry their own Name/Description.)
type Rule struct {
	ID         string      `json:"id"`
	Trigger    TriggerType `json:"trigger"`
	Blueprint  string      `json:"blueprint,omitempty"`
	// Blueprints, when set, makes entity_killed / entity_eliminated act on the
	// combined count of every listed blueprint (e.g. all xeno life stages).
	// Takes precedence over Blueprint.
	Blueprints []string `json:"blueprints,omitempty"`
	Structure  string      `json:"structure,omitempty"`
	Settlement string      `json:"settlement,omitempty"`
	TechKey    string      `json:"tech_key,omitempty"`
	Resource   string      `json:"resource,omitempty"`
	// Target identifies the tagged quest target for target_killed.
	Target     string      `json:"target,omitempty"`
	Threshold  int         `json:"threshold,omitempty"`
	Op         string      `json:"op,omitempty"`
	When       []Condition `json:"when,omitempty"`
}

type RuleSet struct {
	Rules []Rule `json:"rules"`
}
