package wincondition

type TriggerType string

const (
	TriggerDaysSurvived        TriggerType = "days_survived"
	TriggerStructureBuilt      TriggerType = "structure_built"
	TriggerTechResearched      TriggerType = "tech_researched"
	TriggerSettlementPopulation TriggerType = "settlement_population"
	TriggerEntityEliminated    TriggerType = "entity_eliminated"
	TriggerColonistEliminated  TriggerType = "colonist_eliminated"
)

type ResultType string

const (
	ResultWin  ResultType = "win"
	ResultLose ResultType = "lose"
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

type Rule struct {
	ID         string      `json:"id"`
	Trigger    TriggerType `json:"trigger"`
	Blueprint  string      `json:"blueprint,omitempty"`
	Structure  string      `json:"structure,omitempty"`
	Settlement string      `json:"settlement,omitempty"`
	TechKey    string      `json:"tech_key,omitempty"`
	Threshold  int         `json:"threshold,omitempty"`
	Op         string      `json:"op,omitempty"`
	When       []Condition `json:"when,omitempty"`
	Result     ResultType  `json:"result"`
	Outcome    string      `json:"outcome"`
	Message    string      `json:"message"`
}

type RuleSet struct {
	Rules []Rule `json:"rules"`
}
