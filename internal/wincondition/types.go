package wincondition

type TriggerType string

const (
	TriggerDaysSurvived   TriggerType = "days_survived"
	TriggerStructureBuilt TriggerType = "structure_built"
	TriggerResearchDone   TriggerType = "research_done"
	TriggerPopulation     TriggerType = "settlement_population"
)

type ResultType string

const (
	ResultWin  ResultType = "win"
	ResultLose ResultType = "lose"
)

type Rule struct {
	ID        string      `json:"id"`
	Trigger   TriggerType `json:"trigger"`
	Blueprint string      `json:"blueprint,omitempty"`
	Structure string      `json:"structure,omitempty"`
	TechKey   string      `json:"tech_key,omitempty"`
	Threshold int         `json:"threshold,omitempty"`
	Op        string      `json:"op,omitempty"`
	Result    ResultType  `json:"result"`
	Outcome   string      `json:"outcome"`
	Message   string      `json:"message"`
}

type RuleSet struct {
	Rules []Rule `json:"rules"`
}
